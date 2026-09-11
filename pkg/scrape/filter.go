package scrape

import (
	"bytes"
	"io"
	"net/url"
	"strconv"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/prometheus/prometheus/model/textparse"
	"tkestack.io/kvass/pkg/metricdrop"
)

// FilterAndStat rewrites scrape payload in one parse, dropping names in dropSet.
func FilterAndStat(jobName string, URL *url.URL, raw []byte, contentType string, rc []*relabel.Config, dropSet *metricdrop.Snapshot) (out []byte, series int64, bodySize int64, failOpen bool, err error) {
	o := FilterAndStatOutcome(jobName, URL, raw, contentType, rc, dropSet)
	return o.Out, o.Series, o.BodySize, o.FailOpen, o.Err
}

// FilterAndStatOutcome is the detailed entry used by sidecar proxy.
// It shares one parse with FilterAndStat.
func FilterAndStatOutcome(jobName string, URL *url.URL, raw []byte, contentType string, rc []*relabel.Config, dropSet *metricdrop.Snapshot) (o FilterOutcome) {
	defer func() {
		if rec := recover(); rec != nil {
			o = FilterOutcome{
				Out:      raw,
				Series:   0,
				BodySize: int64(len(raw)),
				FailOpen: true,
				Reason:   FailOpenReasonInternalPanic,
				Err:      nil,
			}
		}
	}()
	if isUnsupportedFormat(contentType) {
		return FilterOutcome{Out: raw, BodySize: int64(len(raw)), FailOpen: true, Reason: FailOpenReasonUnsupportedFormat}
	}
	if dropSet == nil || !dropSet.Enabled || len(dropSet.Names) == 0 {
		if URL == nil {
			return FilterOutcome{Out: raw, BodySize: int64(len(raw))}
		}
		total, _, serr := StatisticSeries(jobName, URL, raw, contentType, rc)
		reason := ""
		if serr != nil {
			reason = classifyParseFailure(contentType)
		}
		return FilterOutcome{Out: raw, Series: total, BodySize: int64(len(raw)), Reason: reason, Err: serr}
	}

	p, _ := textparse.New(raw, contentType, false, nil)
	var targetInfo *TargetInfo
	if MetricCollector != nil && URL != nil {
		targetInfo = MetricCollector.GetTargetInfo(URL.Host, URL.Path)
	}

	type meta struct {
		help []byte
		typ  []byte
		unit []byte
	}
	pending := map[string]*meta{}
	emittedMeta := map[string]bool{}
	kept := map[string]bool{}
	outBuf := bytes.NewBuffer(make([]byte, 0, len(raw)))
	var series int64
	// Advance through the input once; rescanning from the beginning for each
	// sample is quadratic and can select a different sample with identical labels.
	lineOffset := 0

	writeNL := func(b []byte) {
		outBuf.Write(b)
		if len(b) == 0 || b[len(b)-1] != '\n' {
			outBuf.WriteByte('\n')
		}
	}

	flushMeta := func(name string) {
		if emittedMeta[name] {
			return
		}
		m := pending[name]
		if m == nil {
			return
		}
		if len(m.help) > 0 {
			outBuf.WriteString("# HELP ")
			outBuf.WriteString(name)
			outBuf.WriteByte(' ')
			writeNL(m.help)
		}
		if len(m.typ) > 0 {
			outBuf.WriteString("# TYPE ")
			outBuf.WriteString(name)
			outBuf.WriteByte(' ')
			writeNL(m.typ)
		}
		if len(m.unit) > 0 {
			outBuf.WriteString("# UNIT ")
			outBuf.WriteString(name)
			outBuf.WriteByte(' ')
			writeNL(m.unit)
		}
		emittedMeta[name] = true
	}

	for {
		et, nerr := p.Next()
		if nerr != nil {
			if nerr == io.EOF {
				break
			}
			return FilterOutcome{
				Out:      raw,
				BodySize: int64(len(raw)),
				FailOpen: true,
				Reason:   classifyParseFailure(contentType),
				Err:      nerr,
			}
		}
		switch et {
		case textparse.EntryHelp:
			nm, h := p.Help()
			name := string(nm)
			if pending[name] == nil {
				pending[name] = &meta{}
			}
			pending[name].help = append([]byte(nil), h...)
		case textparse.EntryType:
			nm, t := p.Type()
			name := string(nm)
			if pending[name] == nil {
				pending[name] = &meta{}
			}
			pending[name].typ = append([]byte(nil), []byte(t)...)
		case textparse.EntryUnit:
			nm, u := p.Unit()
			name := string(nm)
			if pending[name] == nil {
				pending[name] = &meta{}
			}
			pending[name].unit = append([]byte(nil), u...)
		case textparse.EntryComment:
			c := p.Comment()
			outBuf.WriteByte('#')
			writeNL(c)
		case textparse.EntrySeries, textparse.EntryHistogram:
			var lset labels.Labels
			_ = p.Metric(&lset)
			name := lset.Get("__name__")
			if MetricCollector != nil {
				if MetricCollector.NeedCollect(jobName) {
					subsystem := ""
					if targetInfo != nil {
						lset = append(lset, targetInfo.Labels...)
						subsystem = lset.Get("subsystem")
						if subsystem == "" {
							subsystem = lset.Get("subsystemId")
						}
						if subsystem == "" {
							subsystem = targetInfo.SubsystemId
						}
					}
					MetricCollector.AddLabels(jobName, name, lset)
					MetricCollector.AddSubsystemInfo(jobName, name, subsystem)
				}
			}
			if dropSet != nil && dropSet.Contains(name) {
				continue
			}
			kept[name] = true
			flushMeta(name)
			if et == textparse.EntryHistogram {
				rawSeries, ts, _, _ := p.Histogram()
				writeNL(append([]byte(nil), rawSeries...))
				_ = ts
			} else {
				sb, ts, val := p.Series()
				line, consumed := originalSeriesLine(raw[lineOffset:], sb)
				lineOffset += consumed
				if len(line) > 0 {
					writeNL(line)
				} else {
					outBuf.Write(sb)
					outBuf.WriteByte(' ')
					outBuf.WriteString(strconv.FormatFloat(val, 'g', -1, 64))
					if ts != nil {
						outBuf.WriteByte(' ')
						outBuf.WriteString(strconv.FormatInt(*ts, 10))
					}
					outBuf.WriteByte('\n')
				}
			}
			lset, keep := relabel.Process(lset, rc...)
			if keep {
				series++
			}
			_ = lset
		}
	}

	if bytes.Contains(raw, []byte("# EOF")) && !bytes.Contains(outBuf.Bytes(), []byte("# EOF")) {
		outBuf.WriteString("# EOF\n")
	}
	result := outBuf.Bytes()
	return FilterOutcome{Out: result, Series: series, BodySize: int64(len(result))}
}

func originalSeriesLine(raw, series []byte) ([]byte, int) {
	consumed := 0
	for len(raw) > 0 {
		end := bytes.IndexByte(raw, '\n')
		if end < 0 {
			end = len(raw)
		}
		line := raw[:end]
		next := end
		if next < len(raw) {
			next++
		}
		consumed += next
		candidate := bytes.TrimLeft(line, " \t")
		if len(series) > 0 && bytes.HasPrefix(candidate, series) && len(candidate) > len(series) && (candidate[len(series)] == ' ' || candidate[len(series)] == '\t') {
			return line, consumed
		}
		raw = raw[next:]
	}
	return nil, consumed
}

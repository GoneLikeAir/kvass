package scrape

import (
	"bytes"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/prometheus/prometheus/model/textparse"
	"tkestack.io/kvass/pkg/metricdrop"
)

// FilterAndStat rewrites scrape payload in one parse, dropping names in dropSet.
func FilterAndStat(jobName string, URL *url.URL, raw []byte, contentType string, rc []*relabel.Config, dropSet *metricdrop.Snapshot) (out []byte, series int64, bodySize int64, failOpen bool, err error) {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "protobuf") || strings.Contains(ct, "delimited") {
		return raw, 0, int64(len(raw)), true, nil
	}
	if dropSet == nil || !dropSet.Enabled || len(dropSet.Names) == 0 {
		if URL == nil {
			return raw, 0, int64(len(raw)), false, nil
		}
		total, _, serr := StatisticSeries(jobName, URL, raw, contentType, rc)
		return raw, total, int64(len(raw)), false, serr
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
			return raw, 0, int64(len(raw)), true, nerr
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
				outBuf.Write(sb)
				outBuf.WriteByte(' ')
				outBuf.WriteString(strconv.FormatFloat(val, 'g', -1, 64))
				if ts != nil {
					outBuf.WriteByte(' ')
					outBuf.WriteString(strconv.FormatInt(*ts, 10))
				}
				outBuf.WriteByte('\n')
			}
			lset, keep := relabel.Process(lset, rc...)
			if keep {
				series++
			}
			_ = lset
		}
	}

	result := outBuf.Bytes()
	return result, series, int64(len(result)), false, nil
}

module tkestack.io/kvass

go 1.15

require (
	github.com/cssivision/reverseproxy v0.0.1
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/gin-contrib/pprof v1.3.0
	github.com/gin-gonic/gin v1.6.3
	github.com/go-kit/kit v0.10.0
	github.com/go-kit/log v0.2.1
	github.com/gobuffalo/packr/v2 v2.2.0
	github.com/grd/statistics v0.0.0-20130405091615-5af75da930c9
	github.com/mitchellh/hashstructure/v2 v2.0.2
	github.com/mroth/weightedrand v0.4.1
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.42.0
	github.com/prometheus/prometheus v0.0.0-20210701113011-642722e5d01a
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/cobra v1.0.0
	github.com/stretchr/testify v1.8.0
	go.etcd.io/etcd v0.0.0-20191023171146-3cf2f69b5738
	go.uber.org/atomic v1.8.0
	golang.org/x/sync v0.0.0-20220722155255-886fb9371eb4
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.21.1
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v0.21.1
)

replace github.com/prometheus/prometheus => ./staging/src/github.com/promethues/prometheus

package main

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	dto "github.com/prometheus/client_model/go"
)

func TestInotifyInstances(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "inotify-instances")
}

var _ = Describe("inotify-instances", func() {
	var (
		watcherFactory  func() (stopper, error)
		watcher         stopper
		exporterFactory func() ([]byte, error)
		exporterOutput  []byte
		metricFamilies  []*dto.MetricFamily
	)

	JustBeforeEach(func() {
		var err error
		watcher, err = watcherFactory()
		Expect(err).ToNot(HaveOccurred())
	})

	JustBeforeEach(func() {
		var err error
		exporterOutput, err = exporterFactory()
		Expect(err).ToNot(HaveOccurred())
		metricFamilies, err = decodeExporterOutput(exporterOutput)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		metricFamilies = nil
		exporterOutput = nil
		if watcher != nil {
			err := watcher.Stop()
			Expect(err).ToNot(HaveOccurred())
			watcher = nil
		}
	})

	assertNoMetricFamily := func() {
		Expect(metricFamilies).Should(HaveLen(0))
	}

	assertMetricFamily := func() {
		Expect(metricFamilies).Should(HaveLen(1))
		Expect(*metricFamilies[0].Name).Should(Equal("inotify_instances"))
		Expect(*metricFamilies[0].Type).Should(Equal(dto.MetricType_GAUGE))
	}

	assertMetricLength := func(length int) {
		Expect(metricFamilies[0].Metric).Should(HaveLen(length))
	}

	assertGaugeValue := func(midx int, value float64) {
		Expect(*metricFamilies[0].Metric[midx].Gauge.Value).Should(BeNumerically("==", value))
	}

	assertLabelPairsContains := func(midx int, name, value string) {
		l := metricFamilies[0].Metric[midx].Label
		lps := make([]labelPair, len(l))
		for i := range l {
			lp := labelPair{
				Name:  *l[i].Name,
				Value: *l[i].Value,
			}
			lps[i] = lp
		}
		ExpectWithOffset(1, lps).To(ContainElement(labelPair{Name: name, Value: value}))
	}

	assertLabelPairsContainName := func(midx int, name string) {
		l := metricFamilies[0].Metric[midx].Label
		names := make([]string, len(l))
		for i := range l {
			names[i] = *l[i].Name
		}
		ExpectWithOffset(1, names).To(ContainElement(name))
	}

	Context("when run as an unprivileged user", func() {
		BeforeEach(func() {
			exporterFactory = func() ([]byte, error) { return runExporterUnprivileged() }
		})

		Context("with one own process holding one inotify instance", func() {
			BeforeEach(func() {
				watcherFactory = func() (stopper, error) { return runWatcherUnprivileged() }
			})

			It("should emit one metric family", func() {
				assertMetricFamily()
			})
			It("should emit one metric", func() {
				assertMetricLength(1)
			})
			It("should have a gauge value of 1.0", func() {
				assertGaugeValue(0, 1.0)
			})
			It("should emit the {uid: 1000} label pair", func() {
				assertLabelPairsContains(0, "uid", "1000")
			})
			It("should emit the {command: fswatch} label pair", func() {
				assertLabelPairsContains(0, "command", "fswatch")
			})
			It("should emit a label pair with name 'pid'", func() {
				assertLabelPairsContainName(0, "pid")
			})
		})

		Context("with two own processes holding one inotify instance each", func() {
			BeforeEach(func() {
				watcherFactory = func() (stopper, error) {
					w1, err := runWatcherUnprivileged()
					if err != nil {
						return nil, err
					}
					w2, err := runWatcherUnprivileged()
					if err != nil {
						w1.Stop()
						return nil, err
					}
					return &multiStopper{s: []stopper{w1, w2}}, nil
				}
			})

			It("should emit one metric family", func() {
				assertMetricFamily()
			})
			It("should emit two metrics", func() {
				assertMetricLength(2)
			})
			It("should have a gauge value of 1.0 on the first metric", func() {
				assertGaugeValue(0, 1.0)
			})
			It("should have a gauge value of 1.0 on the second metric", func() {
				assertGaugeValue(1, 1.0)
			})
			It("should emit the {uid: 1000} label pair on the first metric", func() {
				assertLabelPairsContains(0, "uid", "1000")
			})
			It("should emit the {uid: 1000} label pair on the second metric", func() {
				assertLabelPairsContains(1, "uid", "1000")
			})
			It("should emit the {command: fswatch} label pair on the first metric", func() {
				assertLabelPairsContains(0, "command", "fswatch")
			})
			It("should emit the {command: fswatch} label pair on the second metric", func() {
				assertLabelPairsContains(1, "command", "fswatch")
			})
			It("should emit a label pair with name 'pid' on the first metric", func() {
				assertLabelPairsContainName(0, "pid")
			})
			It("should emit a label pair with name 'pid' on the second metric", func() {
				assertLabelPairsContainName(1, "pid")
			})
		})

		Context("with one root process holding an inotify instance", func() {
			BeforeEach(func() {
				watcherFactory = func() (stopper, error) { return runWatcherPrivileged() }
			})

			It("should emit no metric families", func() {
				assertNoMetricFamily()
			})
		})
	})

	Context("when run as root", func() {
		BeforeEach(func() {
			exporterFactory = func() ([]byte, error) { return runExporterPrivileged() }
		})

		Context("with one own process holding one inotify instance", func() {
			BeforeEach(func() {
				watcherFactory = func() (stopper, error) { return runWatcherPrivileged() }
			})

			It("should emit one metric family", func() {
				assertMetricFamily()
			})
			It("should emit one metric", func() {
				assertMetricLength(1)
			})
			It("should have a gauge value of 1.0", func() {
				assertGaugeValue(0, 1.0)
			})
			It("should emit the {uid: 0} label pair", func() {
				assertLabelPairsContains(0, "uid", "0")
			})
			It("should emit the {command: fswatch} label pair", func() {
				assertLabelPairsContains(0, "command", "fswatch")
			})
			It("should emit a label pair with name 'pid'", func() {
				assertLabelPairsContainName(0, "pid")
			})
		})

		Context("with two own processes holding one inotify instance each", func() {
			BeforeEach(func() {
				watcherFactory = func() (stopper, error) {
					w1, err := runWatcherPrivileged()
					if err != nil {
						return nil, err
					}
					w2, err := runWatcherPrivileged()
					if err != nil {
						w1.Stop()
						return nil, err
					}
					return &multiStopper{s: []stopper{w1, w2}}, nil
				}
			})

			It("should emit one metric family", func() {
				assertMetricFamily()
			})
			It("should emit two metrics", func() {
				assertMetricLength(2)
			})
			It("should have a gauge value of 1.0 on the first metric", func() {
				assertGaugeValue(0, 1.0)
			})
			It("should have a gauge value of 1.0 on the second metric", func() {
				assertGaugeValue(1, 1.0)
			})
			It("should emit the {uid: 0} label pair on the first metric", func() {
				assertLabelPairsContains(0, "uid", "0")
			})
			It("should emit the {uid: 0} label pair on the second metric", func() {
				assertLabelPairsContains(1, "uid", "0")
			})
			It("should emit the {command: fswatch} label pair on the first metric", func() {
				assertLabelPairsContains(0, "command", "fswatch")
			})
			It("should emit the {command: fswatch} label pair on the second metric", func() {
				assertLabelPairsContains(1, "command", "fswatch")
			})
			It("should emit a label pair with name 'pid' on the first metric", func() {
				assertLabelPairsContainName(0, "pid")
			})
			It("should emit a label pair with name 'pid' on the second metric", func() {
				assertLabelPairsContainName(1, "pid")
			})
		})

		Context("with one unprivileged process holding an inotify instance", func() {
			BeforeEach(func() {
				watcherFactory = func() (stopper, error) { return runWatcherUnprivileged() }
			})

			It("should emit one metric family", func() {
				assertMetricFamily()
			})
			It("should emit one metric", func() {
				assertMetricLength(1)
			})
			It("should have a gauge value of 1.0", func() {
				assertGaugeValue(0, 1.0)
			})
			It("should emit the {uid: 1000} label pair", func() {
				assertLabelPairsContains(0, "uid", "1000")
			})
			It("should emit the {command: fswatch} label pair", func() {
				assertLabelPairsContains(0, "command", "fswatch")
			})
			It("should emit a label pair with name 'pid'", func() {
				assertLabelPairsContainName(0, "pid")
			})
		})
	})
})

type labelPair struct {
	Name  string
	Value string
}

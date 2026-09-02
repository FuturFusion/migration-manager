// Package metrics renders daemon metrics using the OpenMetrics text format.
package metrics

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// Labels are the label names and values attached to a sample.
type Labels map[string]string

// Metric describes a metric family.
type Metric struct {
	name string
	kind string
	help string
}

type sample struct {
	labels Labels
	value  float64
}

// MetricSet holds the samples to report.
type MetricSet struct {
	samples map[Metric][]sample
	suffix  []byte
}

// NewMetricSet returns an empty MetricSet.
func NewMetricSet() *MetricSet {
	return &MetricSet{samples: map[Metric][]sample{}}
}

// Add adds a sample of the given metric, with nil labels for a metric that has none.
func (m *MetricSet) Add(metric Metric, value float64, labels Labels) {
	m.samples[metric] = append(m.samples[metric], sample{labels: labels, value: value})
}

// AddCounts adds one sample per distinct label set, counting the items sharing it.
func AddCounts[T any](metricSet *MetricSet, metric Metric, items []T, labelsOf func(T) Labels) {
	counts := map[string]float64{}
	labels := map[string]Labels{}

	for _, item := range items {
		itemLabels := labelsOf(item)
		key := formatLabels(itemLabels)

		counts[key]++
		labels[key] = itemLabels
	}

	for key, count := range counts {
		metricSet.Add(metric, count, labels[key])
	}
}

// AddRaw appends already formatted metrics to the end of the output.
func (m *MetricSet) AddRaw(rawData []byte) {
	if len(rawData) == 0 {
		return
	}

	m.suffix = append(m.suffix, rawData...)
	if m.suffix[len(m.suffix)-1] != '\n' {
		m.suffix = append(m.suffix, '\n')
	}
}

// String renders the MetricSet using the OpenMetrics text format.
func (m *MetricSet) String() string {
	var out strings.Builder

	families := slices.SortedFunc(maps.Keys(m.samples), func(a Metric, b Metric) int {
		return strings.Compare(a.name, b.name)
	})

	for _, metric := range families {
		fmt.Fprintf(&out, "# HELP %s %s\n", metric.name, metric.help)
		fmt.Fprintf(&out, "# TYPE %s %s\n", metric.name, metric.kind)

		// Samples are sorted so that repeated scrapes report them in the same order.
		lines := make([]string, 0, len(m.samples[metric]))
		for _, sample := range m.samples[metric] {
			value := strconv.FormatFloat(sample.value, 'g', -1, 64)

			labels := formatLabels(sample.labels)
			if labels == "" {
				lines = append(lines, fmt.Sprintf("%s %s\n", metric.name, value))
				continue
			}

			lines = append(lines, fmt.Sprintf("%s{%s} %s\n", metric.name, labels, value))
		}

		slices.Sort(lines)
		out.WriteString(strings.Join(lines, ""))
	}

	out.Write(m.suffix)
	out.WriteString("# EOF\n")

	return out.String()
}

// formatLabels renders the labels of a sample, sorted by label name.
func formatLabels(labels Labels) string {
	entries := make([]string, 0, len(labels))
	for _, name := range slices.Sorted(maps.Keys(labels)) {
		entries = append(entries, fmt.Sprintf(`%s="%s"`, name, labelValueEscaper.Replace(labels[name])))
	}

	return strings.Join(entries, ",")
}

// labelValueEscaper escapes the characters that can't appear as-is in a label value.
var labelValueEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)

package observability

// No-op metrics to remove Prometheus dependency while keeping code paths intact
type nopCounter struct{}
func (nopCounter) Inc() {}

type nopCounterVec struct{}
func (nopCounterVec) WithLabelValues(_ ...string) nopCounter { return nopCounter{} }

type nopObserver struct{}
func (nopObserver) Observe(_ float64) {}

type nopHistogramVec struct{}
func (nopHistogramVec) WithLabelValues(_ ...string) nopObserver { return nopObserver{} }

type nopGauge struct{}
func (nopGauge) Set(_ float64) {}

type Metrics struct {
    HTTPRequestsTotal      nopCounterVec
    HTTPRequestDuration    nopHistogramVec
    MessagesProcessedTotal nopCounterVec
    ActiveConnections      nopGauge
    CreditOperationsTotal  nopCounterVec
    QueueDepth             nopGauge
    RetryAttemptsTotal     nopCounterVec
}

func NewMetrics() *Metrics { return &Metrics{} }

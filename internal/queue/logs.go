package queue

import (
	"context"
	"log/slog"
	"time"

	"github.com/mistryviral05/dockyard/internal/http/modules/deployment"
	"github.com/mistryviral05/dockyard/internal/http/utils/centrifugo"
	"github.com/mistryviral05/dockyard/internal/http/utils/gh"
)

// Log line ke type. deployment_logs ke log_type enum se match karte hain,
// taki inhe seedha db me daala ja sake jab persistence aaye.
const (
	LogTypeBuild  = "build"
	LogTypeSystem = "system"
)

// Ek hi channel pe do tarah ke messages jaate hain, isliye har message me
// event hai: lines log viewer me jaati hain, status timeline aur cache ko.
const (
	EventLog    = "log"
	EventStatus = "status"
)

// logMessage wahi shape hai jo dashboard ko milta hai.
// seq isliye hai ki client out of order ya duplicate messages pehchan sake -
// reconnect pe centrifugo history dobara bhej deta hai.
type logMessage struct {
	Event string   `json:"event"`
	Seq   int      `json:"seq"`
	Type  string   `json:"type"`
	Lines []string `json:"lines"`
	At    string   `json:"at"`
}

// statusMessage har transition pe jaata hai, taki client polling kiye bina
// timeline aage badha sake aur terminal status pe apna cache saaf kar sake.
type statusMessage struct {
	Event  string `json:"event"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	At     string `json:"at"`
}

// logStream build ki lines batch karke Centrifugo pe bhejta hai.
//
// Do wajah se ye alag goroutine me chalta hai:
// ek build ki 2000+ lines me har line pe HTTP call build ko hi dheema kar
// degi, aur namespace ki history 500 messages ki hai - 20 lines per message
// bhejne se wahi history 10,000 lines cover kar leti hai.
type logStream struct {
	lines   chan string
	done    chan struct{}
	client  *centrifugo.Client
	channel string
	logType string
	logger  *slog.Logger
	repo    deployment.Repository

	deploymentId string
	// secrets har line se hataye jaate hain: ye lines ab user ke browser me
	// jaati hain, sirf hamare slog me nahi
	secrets []string

	// lastAt har line ko alag aur badhta hua waqt deta hai. Windows pe
	// time.Now ki granularity milliseconds ki ho sakti hai, isliye ek hi
	// batch ki lines same timestamp le leti aur (logged_at, id) wali
	// ordering build ke asli kram se alag ho jaati.
	lastAt time.Time
}

func (s *Service) newLogStream(deploymentId, logType string) *logStream {
	l := &logStream{
		lines:        make(chan string, logBufferSize),
		done:         make(chan struct{}),
		client:       s.centrifugo,
		channel:      centrifugo.DeploymentChannel(deploymentId),
		logType:      logType,
		logger:       s.logger,
		repo:         s.repo,
		deploymentId: deploymentId,
		secrets:      []string{s.registry.RegistryToken},
	}

	go l.run()

	return l
}

// nextAt strictly badhta hua timestamp deta hai.
// Sirf run() goroutine se call hota hai, isliye lock ki zaroorat nahi.
func (l *logStream) nextAt() time.Time {
	now := time.Now().UTC()
	if !now.After(l.lastAt) {
		now = l.lastAt.Add(time.Microsecond)
	}
	l.lastAt = now
	return now
}

// Add ek line queue karta hai. Ye kabhi block nahi karta: buffer bhar gaya to
// line gir jaati hai. Logs dikhana build ke chalne se kam zaroori hai.
func (l *logStream) Add(line string) {
	for _, secret := range l.secrets {
		line = gh.Redact(line, secret)
	}

	select {
	case l.lines <- line:
	default:
	}
}

// Close bacha hua batch bhejta hai aur publisher ke rukne ka intezaar karta hai.
func (l *logStream) Close() {
	close(l.lines)
	<-l.done
}

func (l *logStream) run() {
	defer close(l.done)

	ticker := time.NewTicker(logFlushInterval)
	defer ticker.Stop()

	storeTicker := time.NewTicker(logStoreFlushInterval)
	defer storeTicker.Stop()

	batch := make([]string, 0, logBatchSize)
	store := make([]deployment.LogEntry, 0, logStoreBatchSize)
	seq := 0

	flush := func() {
		if len(batch) == 0 {
			return
		}
		seq++
		l.publish(seq, batch)
		batch = batch[:0]
	}

	flushStore := func() {
		if len(store) == 0 {
			return
		}
		l.persist(store)
		store = store[:0]
	}

	for {
		select {
		case line, ok := <-l.lines:
			if !ok {
				flush()
				flushStore()
				return
			}

			batch = append(batch, line)
			if len(batch) >= logBatchSize {
				flush()
			}

			store = append(store, deployment.LogEntry{
				Type:    l.logType,
				Message: line,
				At:      l.nextAt(),
			})
			if len(store) >= logStoreBatchSize {
				flushStore()
			}

		case <-ticker.C:
			// build ke lambe chup raaste me bhi user ko kuch dikhta rahe
			flush()

		case <-storeTicker.C:
			flushStore()
		}
	}
}

// persist batch ko db me daalta hai. Publish ki tarah yahan bhi fail hona
// build fail hona nahi hai: live logs to ja hi chuke hain, sirf history
// adhoori rehti hai.
func (l *logStream) persist(entries []deployment.LogEntry) {
	ctx, cancel := context.WithTimeout(context.Background(), logStoreTimeout)
	defer cancel()

	if err := l.repo.InsertLogs(ctx, l.deploymentId, entries); err != nil {
		l.logger.Warn("could not save the build logs",
			slog.String("deployment_id", l.deploymentId),
			slog.Int("lines", len(entries)),
			slog.String("error", err.Error()),
		)
	}
}

func (l *logStream) publish(seq int, lines []string) {
	// build ka apna ctx cancel ho chuka ho sakta hai, par aakhri lines
	// (jaise failure ki wajah) tab bhi user tak jaani chahiye
	ctx, cancel := context.WithTimeout(context.Background(), logPublishTimeout)
	defer cancel()

	msg := logMessage{
		Event: EventLog,
		Seq:   seq,
		Type:  l.logType,
		Lines: append([]string(nil), lines...),
		At:    time.Now().UTC().Format(time.RFC3339Nano),
	}

	if err := l.client.Publish(ctx, l.channel, msg); err != nil {
		// publish fail hona build fail hona nahi hai, sirf note karo
		l.logger.Warn("could not publish the build logs",
			slog.String("deployment_id", l.deploymentId),
			slog.String("channel", l.channel),
			slog.String("error", err.Error()),
		)
	}
}

// publishStatus deployment ka naya status bhejta hai. errMessage sirf failed
// pe bharta hai. Fail hona yahan bhi build fail hona nahi hai: status db me
// ja hi chuka hai, client refresh pe sahi state le lega.
func (s *Service) publishStatus(deploymentId, status, errMessage string) {
	ctx, cancel := context.WithTimeout(context.Background(), logPublishTimeout)
	defer cancel()

	msg := statusMessage{
		Event:  EventStatus,
		Status: status,
		Error:  errMessage,
		At:     time.Now().UTC().Format(time.RFC3339Nano),
	}

	if err := s.centrifugo.Publish(ctx, centrifugo.DeploymentChannel(deploymentId), msg); err != nil {
		s.logger.Warn("could not publish the deployment status",
			slog.String("deployment_id", deploymentId),
			slog.String("status", status),
			slog.String("error", err.Error()),
		)
	}
}

// publishSystemLine ek akeli line bhejta hai - wo steps jo command ke output
// se nahi aate, jaise "cloning" ya "build failed". Ye lines db me bhi jaati
// hain, warna failure ki wajah page refresh karte hi gayab ho jaati.
func (s *Service) publishSystemLine(deploymentId, line string) {
	ctx, cancel := context.WithTimeout(context.Background(), logPublishTimeout)
	defer cancel()

	line = gh.Redact(line, s.registry.RegistryToken)
	at := time.Now().UTC()

	msg := logMessage{
		Event: EventLog,
		Type:  LogTypeSystem,
		Lines: []string{line},
		At:    at.Format(time.RFC3339Nano),
	}

	if err := s.centrifugo.Publish(ctx, centrifugo.DeploymentChannel(deploymentId), msg); err != nil {
		s.logger.Warn("could not publish the deployment status line",
			slog.String("deployment_id", deploymentId),
			slog.String("error", err.Error()),
		)
	}

	entries := []deployment.LogEntry{{Type: LogTypeSystem, Message: line, At: at}}
	if err := s.repo.InsertLogs(ctx, deploymentId, entries); err != nil {
		s.logger.Warn("could not save the deployment status line",
			slog.String("deployment_id", deploymentId),
			slog.String("error", err.Error()),
		)
	}
}

package queue

import (
	"context"
	"log/slog"
	"time"

	"github.com/mistryviral05/dockyard/internal/http/utils/centrifugo"
	"github.com/mistryviral05/dockyard/internal/http/utils/gh"
)

// Log line ke type. deployment_logs ke log_type enum se match karte hain,
// taki inhe seedha db me daala ja sake jab persistence aaye.
const (
	LogTypeBuild  = "build"
	LogTypeSystem = "system"
)

// logMessage wahi shape hai jo dashboard ko milta hai.
// seq isliye hai ki client out of order ya duplicate messages pehchan sake -
// reconnect pe centrifugo history dobara bhej deta hai.
type logMessage struct {
	Seq   int      `json:"seq"`
	Type  string   `json:"type"`
	Lines []string `json:"lines"`
	At    string   `json:"at"`
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

	deploymentId string
	// secrets har line se hataye jaate hain: ye lines ab user ke browser me
	// jaati hain, sirf hamare slog me nahi
	secrets []string
}

func (s *Service) newLogStream(deploymentId, logType string) *logStream {
	l := &logStream{
		lines:        make(chan string, logBufferSize),
		done:         make(chan struct{}),
		client:       s.centrifugo,
		channel:      centrifugo.DeploymentChannel(deploymentId),
		logType:      logType,
		logger:       s.logger,
		deploymentId: deploymentId,
		secrets:      []string{s.registry.RegistryToken},
	}

	go l.run()

	return l
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

	batch := make([]string, 0, logBatchSize)
	seq := 0

	flush := func() {
		if len(batch) == 0 {
			return
		}
		seq++
		l.publish(seq, batch)
		batch = batch[:0]
	}

	for {
		select {
		case line, ok := <-l.lines:
			if !ok {
				flush()
				return
			}
			batch = append(batch, line)
			if len(batch) >= logBatchSize {
				flush()
			}

		case <-ticker.C:
			// build ke lambe chup raaste me bhi user ko kuch dikhta rahe
			flush()
		}
	}
}

func (l *logStream) publish(seq int, lines []string) {
	// build ka apna ctx cancel ho chuka ho sakta hai, par aakhri lines
	// (jaise failure ki wajah) tab bhi user tak jaani chahiye
	ctx, cancel := context.WithTimeout(context.Background(), logPublishTimeout)
	defer cancel()

	msg := logMessage{
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

// publishSystemLine ek akeli line bhejta hai - wo steps jo command ke output
// se nahi aate, jaise "cloning" ya "build failed".
func (s *Service) publishSystemLine(deploymentId, line string) {
	ctx, cancel := context.WithTimeout(context.Background(), logPublishTimeout)
	defer cancel()

	msg := logMessage{
		Type:  LogTypeSystem,
		Lines: []string{gh.Redact(line, s.registry.RegistryToken)},
		At:    time.Now().UTC().Format(time.RFC3339Nano),
	}

	if err := s.centrifugo.Publish(ctx, centrifugo.DeploymentChannel(deploymentId), msg); err != nil {
		s.logger.Warn("could not publish the deployment status line",
			slog.String("deployment_id", deploymentId),
			slog.String("error", err.Error()),
		)
	}
}

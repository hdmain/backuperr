package host

import (
	"log"
)

// receiveProgressLog is used with io.TeeReader while persisting the upload bundle.
type receiveProgressLog struct {
	log        *log.Logger
	clientIP   string
	backupType string
	parentID   string
	written    int64
	lastStep   int64
}

func newReceiveProgressLog(l *log.Logger, clientIP, backupType, parentID string) *receiveProgressLog {
	return &receiveProgressLog{
		log:        l,
		clientIP:   clientIP,
		backupType: backupType,
		parentID:   parentID,
	}
}

func (w *receiveProgressLog) Write(p []byte) (int, error) {
	n := int64(len(p))
	w.written += n
	const step = 32 << 20 // log every 32 MiB while receiving
	milestone := (w.written / step) * step
	if milestone > w.lastStep {
		w.log.Printf("receive backup: client=%s type=%s parent=%q bundle_bytes_received=%d",
			w.clientIP, w.backupType, w.parentID, w.written)
		w.lastStep = milestone
	}
	return len(p), nil
}

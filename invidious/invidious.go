package invidious

import (
	"bytes"
	"time"

	"github.com/birabittoh/myks"
	"github.com/birabittoh/rabbitpipe"
	"github.com/sirupsen/logrus"
)

var logger *logrus.Logger
var buffers *myks.KeyStore[VideoBuffer]
var RP *rabbitpipe.Client

type VideoBuffer struct {
	Buffer *bytes.Buffer
	Length int64
}

func Init(logLevel logrus.Level, instance string) {
	logger = logrus.New()
	logger.SetLevel(logLevel)

	buffers = myks.New[VideoBuffer](time.Minute)

	RP = rabbitpipe.New(instance)
}

func GetVideoURL(video rabbitpipe.Video) string {
	if len(video.FormatStreams) == 0 {
		return ""
	}
	return video.FormatStreams[0].URL
}

func NewVideoBuffer(b *bytes.Buffer, l int64) *VideoBuffer {
	d := new(bytes.Buffer)
	d.Write(b.Bytes())

	return &VideoBuffer{Buffer: d, Length: l}
}

func (vb *VideoBuffer) Clone() *VideoBuffer {
	return NewVideoBuffer(vb.Buffer, vb.Length)
}

func (vb *VideoBuffer) ValidateLength() bool {
	return vb.Length > 0 && vb.Length == int64(vb.Buffer.Len())
}

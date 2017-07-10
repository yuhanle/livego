package rtmprelay

import (
	"errors"
	"fmt"
	"github.com/livego/protocol/httpflvclient"
	"github.com/livego/protocol/rtmp/core"
	"log"
)

type FlvPull struct {
	FlvUrl        string
	RtmpUrl       string
	flvclient     *httpflvclient.HttpFlvClient
	rtmpclient    *core.ConnClient
	isStart       bool
	csChan        chan *core.ChunkStream
	isFlvHdrReady bool
	databuffer    []byte
	dataNeedLen   int
	testFlag      bool
}

const FLV_HEADER_LENGTH = 13

func NewFlvPull(flvurl *string, rtmpurl *string) *FlvPull {
	return &FlvPull{
		FlvUrl:  *flvurl,
		RtmpUrl: *rtmpurl,
		isStart: false,
	}
}

func (self *FlvPull) HandleFlvData(packet []byte) error {
	messagetype := packet[0]
	payloadLen := int(packet[1])<<16 + int(packet[2])<<8 + int(packet[3])
	timestamp := int(packet[4])<<16 + int(packet[5])<<8 + int(packet[6]) + int(packet[7])<<24
	streamid := int(packet[8])<<16 + int(packet[9])<<8 + int(packet[10])

	if messagetype == 0x09 {
		if packet[11] == 0x17 && packet[12] == 0x00 {
			log.Printf("it's pps and sps: messagetype=%d, payloadlen=%d, timestamp=%d, streamid=%d", messagetype, payloadLen, timestamp, streamid)

		} else if packet[11] == 0x17 && packet[12] == 0x01 {
			log.Printf("it's I frame: messagetype=%d, payloadlen=%d, timestamp=%d, streamid=%d", messagetype, payloadLen, timestamp, streamid)
		} else if packet[11] == 0x27 {
			log.Printf("it's P frame: messagetype=%d, payloadlen=%d, timestamp=%d, streamid=%d", messagetype, payloadLen, timestamp, streamid)
		}
	} else if messagetype == 0x08 {
		log.Printf("it's audio: messagetype=%d, payloadlen=%d, timestamp=%d, streamid=%d", messagetype, payloadLen, timestamp, streamid)
	} else if messagetype == 0x18 {
		log.Printf("it's metadata: messagetype=%d, payloadlen=%d, timestamp=%d, streamid=%d", messagetype, payloadLen, timestamp, streamid)
	}

	return nil
}

func (self *FlvPull) sendPublishChunkStream() {

}

func (self *FlvPull) Start() error {
	if self.isStart {
		errString := fmt.Sprintf("NewHttpFlvClient(%s->%s) has already started.", self.FlvUrl, self.RtmpUrl)
		return errors.New(errString)
	}
	self.flvclient = httpflvclient.NewHttpFlvClient(self.FlvUrl)
	if self.flvclient == nil {
		errString := fmt.Sprintf("NewHttpFlvClient(%s) error", self.FlvUrl)
		return errors.New(errString)
	}

	self.rtmpclient = core.NewConnClient()

	self.csChan = make(chan *core.ChunkStream)

	self.isFlvHdrReady = false
	self.databuffer = nil
	err := self.flvclient.Start(self)
	if err != nil {
		log.Printf("flvclient start error:%v", err)
		close(self.csChan)
		return err
	}

	err = self.rtmpclient.Start(self.RtmpUrl, "publish")
	if err != nil {
		log.Printf("rtmpclient.Start url=%v error", self.RtmpUrl)
		self.flvclient.Stop()
		close(self.csChan)
		return err
	}

	self.isStart = true

	go self.sendPublishChunkStream()

	return nil
}

func (self *FlvPull) Stop() {
	if self.isStart {
		log.Printf("NewHttpFlvClient(%s->%s) has already stoped.", self.FlvUrl, self.RtmpUrl)
		return
	}

	self.flvclient.Stop()
	self.rtmpclient.Close(nil)

	self.isStart = false

	close(self.csChan)
}

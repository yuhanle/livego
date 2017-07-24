package rtmprelay

import (
	"errors"
	"fmt"
	"github.com/livego/av"
	log "github.com/livego/logging"
	"github.com/livego/protocol/httpflvclient"
	"github.com/livego/protocol/rtmp/core"
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
		csChan:  make(chan *core.ChunkStream, 1000),
	}
}

func (self *FlvPull) HandleFlvData(packet []byte) error {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("HandleFlvData cs channel has already been closed:%v", e)
			return
		}
	}()
	var cs *core.ChunkStream

	cs = &core.ChunkStream{}
	messagetype := packet[0]
	payloadLen := int(packet[1])<<16 + int(packet[2])<<8 + int(packet[3])
	timestamp := int(packet[4])<<16 + int(packet[5])<<8 + int(packet[6]) + int(packet[7])<<24
	streamid := int(packet[8])<<16 + int(packet[9])<<8 + int(packet[10])

	if messagetype == 0x09 {
		if packet[11] == 0x17 && packet[12] == 0x00 {
			//log.Printf("it's pps and sps: messagetype=%d, payloadlen=%d, timestamp=%d, streamid=%d", messagetype, payloadLen, timestamp, streamid)
			cs.TypeID = av.TAG_VIDEO
		} else if packet[11] == 0x17 && packet[12] == 0x01 {
			//log.Printf("it's I frame: messagetype=%d, payloadlen=%d, timestamp=%d, streamid=%d", messagetype, payloadLen, timestamp, streamid)
			cs.TypeID = av.TAG_VIDEO
		} else if packet[11] == 0x27 {
			cs.TypeID = av.TAG_VIDEO
			//log.Printf("it's P frame: messagetype=%d, payloadlen=%d, timestamp=%d, streamid=%d", messagetype, payloadLen, timestamp, streamid)
		}
	} else if messagetype == 0x08 {
		cs.TypeID = av.TAG_AUDIO
		//log.Printf("it's audio: messagetype=%d, payloadlen=%d, timestamp=%d, streamid=%d", messagetype, payloadLen, timestamp, streamid)
	} else if messagetype == 0x12 {
		cs.TypeID = av.MetadatAMF0
		//log.Printf("it's metadata: messagetype=%d, payloadlen=%d, timestamp=%d, streamid=%d", messagetype, payloadLen, timestamp, streamid)
	} else if messagetype == 0xff {
		cs.TypeID = av.MetadataAMF3
	}

	cs.Data = packet[11:]
	cs.Length = uint32(payloadLen)
	cs.StreamID = uint32(streamid)
	cs.Timestamp = uint32(timestamp)

	if uint32(payloadLen) != cs.Length {
		errString := fmt.Sprintf("payload length(%d) is not equal to data length(%d)",
			payloadLen, cs.Length)
		return errors.New(errString)
	}

	self.csChan <- cs
	return nil
}

func (self *FlvPull) sendPublishChunkStream() {

	for {
		csPacket, ok := <-self.csChan
		if ok {
			self.rtmpclient.Write(*csPacket)
			//log.Printf("type=%d, length=%d, timestamp=%d, error=%v",
			//	csPacket.TypeID, csPacket.Length, csPacket.Timestamp, err)
		} else {
			break
		}
	}

	log.Info("sendPublishChunkStream is ended.")
}

func (self *FlvPull) Start() error {
	if self.isStart {
		errString := fmt.Sprintf("FlvPull(%s->%s) has already started.", self.FlvUrl, self.RtmpUrl)
		return errors.New(errString)
	}
	self.flvclient = httpflvclient.NewHttpFlvClient(self.FlvUrl)
	if self.flvclient == nil {
		errString := fmt.Sprintf("FlvPull(%s) error", self.FlvUrl)
		return errors.New(errString)
	}

	self.rtmpclient = core.NewConnClient()

	self.csChan = make(chan *core.ChunkStream)

	self.isFlvHdrReady = false
	self.databuffer = nil
	err := self.flvclient.Start(self)
	if err != nil {
		log.Errorf("flvclient start error:%v", err)
		close(self.csChan)
		return err
	}

	err = self.rtmpclient.Start(self.RtmpUrl, "publish")
	if err != nil {
		log.Errorf("rtmpclient.Start url=%v error", self.RtmpUrl)
		self.flvclient.Stop()
		close(self.csChan)
		return err
	}

	self.isStart = true

	go self.sendPublishChunkStream()

	return nil
}

func (self *FlvPull) Stop() {
	if !self.isStart {
		log.Errorf("FlvPull(%s->%s) has already stoped.", self.FlvUrl, self.RtmpUrl)
		return
	}

	self.flvclient.Stop()
	self.rtmpclient.Close(nil)

	self.isStart = false

	close(self.csChan)
	log.Infof("FlvPull(%s->%s) stoped.", self.FlvUrl, self.RtmpUrl)
}

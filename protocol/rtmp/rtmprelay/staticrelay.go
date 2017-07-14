package rtmprelay

import (
	"errors"
	"fmt"
	"github.com/livego/av"
	"github.com/livego/configure"
	"github.com/livego/protocol/rtmp/core"
	"log"
	"strconv"
	"strings"
	"sync"
)

type StaticPush struct {
	RtmpUrl                string
	RtmpSubUrls            [2]string
	packet_chan            chan *av.Packet
	sndctrl_chan           chan string
	connectClient          *core.ConnClient
	startflag              bool
	lastIFrameTimestamp    uint32
	lastSubIFrameTimestamp [2]uint32
}

var G_StaticPushMap = make(map[string](*StaticPush))
var g_MapLock = new(sync.RWMutex)

var (
	STATIC_RELAY_STOP_CTRL = "STATIC_RTMPRELAY_STOP"
)

const TIMESTAMP_SEND_INTERVAL = 2500

func GetStaticPushList(url string) ([]string, error) {
	pushurlList, ok := configure.GetStaticPushUrlList(url)

	if !ok {
		return nil, errors.New("no static push url")
	}

	return pushurlList, nil
}

func GetIndexbySuburl(subrtmpurl string) int {
	retIndex := -1
	argArray := strings.Split(subrtmpurl, "/")

	if len(argArray) < 2 {
		return retIndex
	}

	argString := argArray[len(argArray)-2]

	foundIndex := strings.Index(argString, "_")

	if foundIndex < 0 {
		return retIndex
	}

	numString := argString[foundIndex-1 : foundIndex]
	retIndex, err := strconv.Atoi(numString)
	if err != nil {
		log.Printf("atoi:%v", err)
		retIndex = -1
	}

	return retIndex
}

func GetStaticPushObjectbySubstream(subrtmpurl string) (int, *StaticPush) {
	subUpstreamUrl := ""

	upstreamPrefixUrl, ok := configure.GetSubStaticMasterPushUrl(subrtmpurl)
	if upstreamPrefixUrl != "" && ok {
		lastIndex := strings.LastIndex(subrtmpurl, "/")
		lastPart := subrtmpurl[lastIndex:]
		subUpstreamUrl = upstreamPrefixUrl + lastPart

		//log.Printf("subUpstreamUrl=%s", subUpstreamUrl)
	}

	subStreamIndex := GetIndexbySuburl(subrtmpurl)
	if subStreamIndex > 2 || subStreamIndex < 0 {
		return -1, nil
	}

	subStreamIndex--
	if subUpstreamUrl != "" {
		staticPushObj, err := GetStaticPushObject(subUpstreamUrl)
		if err == nil && staticPushObj != nil {
			//log.Printf("GetStaticPushObjectbySubstream: upstream=%s, substream=%s",
			//	subUpstreamUrl, staticPushObj.RtmpSubUrls[subStreamIndex])
			if staticPushObj.RtmpSubUrls[subStreamIndex] == "" || staticPushObj.RtmpSubUrls[subStreamIndex] == subrtmpurl {
				return subStreamIndex, staticPushObj
			}
		}
	}
	return -1, nil
}

func GetAndCreateStaticPushObject(rtmpurl string) *StaticPush {
	g_MapLock.RLock()
	staticpush, ok := G_StaticPushMap[rtmpurl]
	log.Printf("GetAndCreateStaticPushObject: %s, return %v", rtmpurl, ok)
	if !ok {
		g_MapLock.RUnlock()
		newStaticpush := NewStaticPush(rtmpurl)

		g_MapLock.Lock()
		G_StaticPushMap[rtmpurl] = newStaticpush
		g_MapLock.Unlock()

		return newStaticpush
	}
	g_MapLock.RUnlock()

	return staticpush
}

func GetStaticPushObject(rtmpurl string) (*StaticPush, error) {
	g_MapLock.RLock()
	if staticpush, ok := G_StaticPushMap[rtmpurl]; ok {
		g_MapLock.RUnlock()
		return staticpush, nil
	}
	g_MapLock.RUnlock()

	return nil, errors.New(fmt.Sprintf("G_StaticPushMap[%s] not exist...."))
}

func ReleaseStaticPushObject(rtmpurl string) {
	g_MapLock.RLock()
	if _, ok := G_StaticPushMap[rtmpurl]; ok {
		g_MapLock.RUnlock()

		log.Printf("ReleaseStaticPushObject %s ok", rtmpurl)
		g_MapLock.Lock()
		delete(G_StaticPushMap, rtmpurl)
		g_MapLock.Unlock()
	} else {
		g_MapLock.RUnlock()
		log.Printf("ReleaseStaticPushObject: not find %s", rtmpurl)
	}
}

func NewStaticPush(rtmpurl string) *StaticPush {
	return &StaticPush{
		RtmpUrl:                rtmpurl,
		RtmpSubUrls:            [2]string{"", ""},
		packet_chan:            make(chan *av.Packet, 500),
		sndctrl_chan:           make(chan string),
		connectClient:          nil,
		startflag:              false,
		lastIFrameTimestamp:    0,
		lastSubIFrameTimestamp: [2]uint32{0, 0},
	}
}

func (self *StaticPush) Start() error {
	if self.startflag {
		return errors.New(fmt.Sprintf("StaticPush already start %s", self.RtmpUrl))
	}

	self.connectClient = core.NewConnClient()

	log.Printf("static publish server addr:%v starting....", self.RtmpUrl)
	err := self.connectClient.Start(self.RtmpUrl, "publish")
	if err != nil {
		log.Printf("connectClient.Start url=%v error", self.RtmpUrl)
		return err
	}
	log.Printf("static publish server addr:%v started, streamid=%d", self.RtmpUrl, self.connectClient.GetStreamId())

	/*
		log.Printf("++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++")
		tempUrl := "rtmp://inke.8686c.com/live/inke_1"
		index := 0
		err = self.connectClient.StartSubStream(tempUrl, index, "publish")
		if err != nil {
			log.Printf("connectClient.StartSubStream index=%d url=%v error=%v", index, tempUrl, err)
			return err
		}
		log.Printf("static publish server addr:%v started, index=%d, streamid=%d", tempUrl, index, self.connectClient.GetSubStreamId(index))
	*/
	go self.HandleAvPacket()

	self.startflag = true
	return nil
}

func (self *StaticPush) StartSubUrl(subrtmpurl string) error {
	if !self.startflag {
		log.Printf("Master StaticPush has not started %s", self.RtmpUrl)
		return errors.New(fmt.Sprintf("Master StaticPush has not started %s", self.RtmpUrl))
	}

	saveIndex := -1
	for index, rtmpurlString := range self.RtmpSubUrls {
		if rtmpurlString == "" {
			saveIndex = index
			break
		}
	}
	if saveIndex == -1 {
		return errors.New(fmt.Sprintf("Master StaticPush has full sub url", self.RtmpUrl))
	}

	err := self.connectClient.StartSubStream(subrtmpurl, saveIndex, "publish")
	if err != nil {
		log.Printf("connectClient.StartSubStream index=%d url=%v error=%v", saveIndex, subrtmpurl, err)
		return err
	}
	self.RtmpSubUrls[saveIndex] = subrtmpurl
	log.Printf("StartSubUrl:%v started, index=%d, streamid=%d", subrtmpurl, saveIndex, self.connectClient.GetSubStreamId(saveIndex))
	return nil
}

func (self *StaticPush) Stop() {
	if !self.startflag {
		return
	}

	log.Printf("StaticPush Stop: %s", self.RtmpUrl)
	self.sndctrl_chan <- STATIC_RELAY_STOP_CTRL
	self.startflag = false
}

func (self *StaticPush) StopSubUrl(subrtmpurl string) {
	saveIndex := -1
	for index, rtmpurlString := range self.RtmpSubUrls {
		if rtmpurlString == subrtmpurl {
			saveIndex = index
			break
		}
	}

	if saveIndex != -1 {
		log.Printf("StopSubUrl: %s", self.RtmpSubUrls[saveIndex])
		self.RtmpSubUrls[saveIndex] = ""
	}
}

func (self *StaticPush) WriteAvPacket(packet *av.Packet) {
	if !self.startflag {
		return
	}

	self.packet_chan <- packet
}

func (self *StaticPush) sendSyncTimestamp(p *av.Packet) {
	if !self.startflag {
		return
	}

	var lasttimestamp uint32
	if p.StreamIndex > 0 { //sub stream
		lasttimestamp = self.lastSubIFrameTimestamp[p.StreamIndex-1]
	} else {
		lasttimestamp = self.lastIFrameTimestamp
	}
	if p.IsVideo {
		packet := p.Data[:]

		//for I frame or timeout
		if (packet[0] == 0x17 && packet[1] == 0x00) || (packet[0] == 0x17 && packet[1] == 0x01) {
			if p.StreamIndex > 0 {
				self.lastSubIFrameTimestamp[p.StreamIndex-1] = p.TimeStamp
				self.connectClient.WriteSubTimestampMeta(int(p.StreamIndex-1), p.TimeStamp)
			} else {
				self.lastIFrameTimestamp = p.TimeStamp
				self.connectClient.WriteTimestampMeta(p.TimeStamp)
			}
		} else if (p.TimeStamp - lasttimestamp) >= TIMESTAMP_SEND_INTERVAL {
			if p.StreamIndex > 0 {
				self.lastSubIFrameTimestamp[p.StreamIndex-1] = p.TimeStamp
				self.connectClient.WriteSubTimestampMeta(int(p.StreamIndex-1), p.TimeStamp)
			} else {
				self.lastIFrameTimestamp = p.TimeStamp
				self.connectClient.WriteTimestampMeta(p.TimeStamp)
			}
		}
	}
}

func (self *StaticPush) sendPacket(p *av.Packet) {
	if !self.startflag {
		return
	}
	var cs core.ChunkStream

	cs.Data = p.Data
	cs.Length = uint32(len(p.Data))

	cs.Timestamp = p.TimeStamp

	if p.StreamIndex > 0 {
		index := p.StreamIndex - 1
		cs.StreamID = self.connectClient.GetSubStreamId(int(index))
	} else {
		cs.StreamID = self.connectClient.GetStreamId()
	}

	self.sendSyncTimestamp(p)
	//cs.Timestamp += v.BaseTimeStamp()

	//log.Printf("Static sendPacket: rtmpurl=%s, length=%d, streamid=%d",
	//	self.RtmpUrl, len(p.Data), cs.StreamID)
	if p.IsVideo {
		cs.TypeID = av.TAG_VIDEO
	} else {
		if p.IsMetadata {
			cs.TypeID = av.TAG_SCRIPTDATAAMF0
		} else {
			cs.TypeID = av.TAG_AUDIO
		}
	}

	self.connectClient.Write(cs)
	/*
		var csSub core.ChunkStream
		csSub.Data = p.Data
		csSub.Length = uint32(len(p.Data))
		csSub.StreamID = self.connectClient.GetSubStreamId(0)
		csSub.Timestamp = p.TimeStamp

		if p.IsVideo {
			csSub.TypeID = av.TAG_VIDEO
		} else {
			if p.IsMetadata {
				csSub.TypeID = av.TAG_SCRIPTDATAAMF0
			} else {
				csSub.TypeID = av.TAG_AUDIO
			}
		}

		self.connectClient.Write(csSub)
	*/
}

func (self *StaticPush) HandleAvPacket() {
	if !self.IsStart() {
		log.Printf("static push %s not started", self.RtmpUrl)
		return
	}

	for {
		select {
		case packet := <-self.packet_chan:
			self.sendPacket(packet)
		case ctrlcmd := <-self.sndctrl_chan:
			if ctrlcmd == STATIC_RELAY_STOP_CTRL {
				self.connectClient.Close(nil)
				log.Printf("Static HandleAvPacket close: publishurl=%s", self.RtmpUrl)
				break
			}
		}
	}
}

func (self *StaticPush) IsStart() bool {
	return self.startflag
}

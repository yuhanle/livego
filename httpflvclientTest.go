package main

import (
	"github.com/livego/protocol/rtmp/rtmprelay"
	"log"
)

/*
type FlvRcvHandle struct {
	version string
}

func (self *FlvRcvHandle) HandleFlvData(data []byte, length int) error {
	return httpflv.WriteFlvFile(data, length)
}
*/
func main() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	/*
		client := httpflv.NewHttpFlvClient("http://pull99.a8.com/live/1499336947298690.flv")

		handle := &FlvRcvHandle{
			version: "flv1.0",
		}
		client.Start(handle)
	*/
	flvurl := "http://pull99.a8.com/live/1499666817758063.flv"
	rtmpurl := "rtmp://alpush.xxxx.cn/live/1499666817758063_test"
	flvPull := rtmprelay.NewFlvPull(&flvurl, &rtmpurl)
	err := flvPull.Start()
	if err != nil {
		return
	}

	defer flvPull.Stop()
	done := make(chan int)

	<-done
}

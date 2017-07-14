package configure

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
)

/*
{
    "listen": 1935,
    "servers":[
        {
        "servername":"live",
	    "static_push":[{"master_prefix":"live", "upstream":"rtmp://inke.8686c.com/"}],
	    "sub_static_push":[{"master_prefix":"live/trans/inke/mlinkm", "sub_prefix":"live/trans/inke/mlinks"}]
        }
    ]
}
*/
type SubStaticPush struct {
	Master_prefix string
	Sub_prefix    string
}

type StaticPushInfo struct {
	Master_prefix string
	Upstream      string
}

type ServerInfo struct {
	Servername      string
	Static_push     []StaticPushInfo
	Sub_static_push []SubStaticPush
}

type ServerCfg struct {
	Listen  int
	Servers []ServerInfo
}

var RtmpServercfg ServerCfg

func LoadConfig(configfilename string) error {
	log.Printf("starting load configure file(%s)......", configfilename)
	data, err := ioutil.ReadFile(configfilename)
	if err != nil {
		log.Printf("ReadFile %s error:%v", configfilename, err)
		return err
	}

	log.Printf("loadconfig: \r\n%s", string(data))

	err = json.Unmarshal(data, &RtmpServercfg)
	if err != nil {
		log.Printf("json.Unmarshal error:%v", err)
		return err
	}
	log.Printf("get config json data:%v", RtmpServercfg)
	return nil
}

func GetStaticPushUrlList(rtmpurl string) (retArray []string, bRet bool) {
	retArray = nil
	bRet = false

	//log.Printf("rtmpurl=%s", rtmpurl)
	url := rtmpurl[7:]

	index := strings.Index(url, "/")
	if index <= 0 {
		return
	}
	url = url[index+1:]
	//log.Printf("GetStaticPushUrlList: url=%s", url)
	for _, serverinfo := range RtmpServercfg.Servers {
		//log.Printf("server info:%v", serverinfo)
		for _, staticpushItem := range serverinfo.Static_push {
			masterPrefix := staticpushItem.Master_prefix
			upstream := staticpushItem.Upstream
			//log.Printf("push item: masterprefix=%s, upstream=%s", masterPrefix, upstream)
			if strings.Contains(url, masterPrefix) {
				destUrl := fmt.Sprintf("%s%s", upstream, url)
				retArray = append(retArray, destUrl)
				bRet = true
			}

		}
	}

	//log.Printf("GetStaticPushUrlList:%v, %v", retArray, bRet)
	return
}

func GetSubStaticMasterPushUrl(rtmpurl string) (retUpstream string, bRet bool) {
	retUpstream = ""
	bRet = false

	url := rtmpurl[7:]

	index := strings.Index(url, "/")
	if index <= 0 {
		return
	}
	url = url[index+1:]

	bFoundFlag := false
	foundMasterPrefix := ""
	for _, serverinfo := range RtmpServercfg.Servers {
		for _, substaticpushItem := range serverinfo.Sub_static_push {
			masterPrefix := substaticpushItem.Master_prefix
			subPrefix := substaticpushItem.Sub_prefix
			if strings.Contains(url, subPrefix) {
				foundMasterPrefix = masterPrefix
				bFoundFlag = true
				break
			}
		}

		if bFoundFlag {
			for _, staticpushItem := range serverinfo.Static_push {
				masterPrefix := staticpushItem.Master_prefix
				upstream := staticpushItem.Upstream
				if foundMasterPrefix == masterPrefix {
					retUpstream = upstream + masterPrefix
					bRet = true
					return
				}
			}
			break
		}
	}

	return
}

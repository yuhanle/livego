package httpflv

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

type HttpFlvClient struct {
	Url      string
	HostUrl  string
	HostPort int
	PathUrl  string
}

type FlvRcvCallback interface {
	HandleFlvData(data []byte, length int) error
}

//http://pull2.a8.com/live/1499323853715657.flv
func NewHttpFlvClient(url string) *HttpFlvClient {

	log.Printf("Http flv client %s", url)
	if len(url) <= 6 {
		log.Printf("url(%s) length(%d) is error", url, len(url))
		return nil
	}

	if url[:7] != "http://" {
		log.Printf("url(%s) header(%s) is error", url, url[:7])
		return nil
	}
	tempString := url[7:]

	pathArray := strings.Split(tempString, "/")

	hostUrl := pathArray[0]

	hostInfoArray := strings.Split(hostUrl, ":")
	log.Printf("host info array=%v", hostInfoArray)

	var hostPort int
	if len(hostInfoArray) == 1 {
		hostPort = 80
	} else {
		hostportString := hostInfoArray[1]
		var err error
		hostPort, err = strconv.Atoi(hostportString)
		if err != nil {
			log.Printf("host port(%s) error=%v", hostportString, err)
			return nil
		}
	}
	log.Printf("host url=%s, hostport=%d", hostUrl, hostPort)

	var pathString string
	for _, pachUrl := range pathArray[1:] {
		pathString = pathString + "/" + pachUrl
	}

	//pathString = pathString[0:(len(pathString) - 1)]

	log.Printf("pathurl=%s", pathString)

	return &HttpFlvClient{
		Url:      url,
		HostUrl:  hostUrl,
		HostPort: hostPort,
		PathUrl:  pathString,
	}
}

//only for test use
func checkFileIsExist(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

//only for test use
func WriteFlvFile(data []byte, length int) error {
	filename := "temp.flv"

	ret, err := checkFileIsExist(filename)
	if err != nil {
		return err
	}

	if ret {
		filehandle, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666) //打开文件
		if err != nil {
			log.Printf("Open file %s error=%v", filename, err)
			return err
		}

		defer filehandle.Close()
		//log.Printf("writeFlvFile(%s): open and write %d bytes", filename, length)
		filehandle.Write(data[:length])
	} else {
		filehandle, err := os.Create(filename)
		if err != nil {
			log.Printf("Create file %s error=%v", filename, err)
			return err
		}

		defer filehandle.Close()
		//log.Printf("writeFlvFile(%s): create and write %d bytes", filename, length)
		filehandle.Write(data[:length])
	}

	return nil
}

func (self *HttpFlvClient) Start(rcvHandle FlvRcvCallback) error {
	hostString := fmt.Sprintf("%s:%d", self.HostUrl, self.HostPort)

	conn, err := net.Dial("tcp", hostString)
	if err != nil {
		log.Printf("HttpFlvClient.Start(%s) Dail error=%v", hostString, err)
		return err
	}

	content := fmt.Sprintf("GET %s HTTP/1.1\r\n", self.PathUrl)
	content = content + fmt.Sprintf("Accept:*/*\r\n")
	content = content + fmt.Sprintf("Accept-Encoding:gzip\r\n")
	content = content + fmt.Sprintf("Accept-Language:zh_CN\r\n")
	content = content + fmt.Sprintf("Connection:Keep-Alive\r\n")
	content = content + fmt.Sprintf("Host:%s\r\n", self.HostUrl)
	content = content + fmt.Sprintf("Referer:http://www.abc.com/vplayer.swf\r\n\r\n")

	log.Printf("send content:\r\n%s", content)
	conn.Write([]byte(content))

	var rcvBuff []byte
	flvBuff := make([]byte, 1024)

	for {
		temp := make([]byte, 1)
		retLen, err := conn.Read(temp)
		if err != nil || retLen <= 0 {
			log.Printf("connect read len=%d, error=%v", retLen, err)
			return errors.New("connect read error")
		}
		rcvBuff = append(rcvBuff, temp[0])

		if len(rcvBuff) >= 4 {
			lastIndex := len(rcvBuff) - 1
			if rcvBuff[lastIndex-3] == 0x0d && rcvBuff[lastIndex-2] == 0x0a && rcvBuff[lastIndex-1] == 0x0d && rcvBuff[lastIndex] == 0x0a {
				break
			}
		}
	}
	log.Printf("rcv http header:\r\n%s", string(rcvBuff))

	go func() {
		totalLen := 0
		log.Printf("rcv data:")
		for {
			retLen, err := conn.Read(flvBuff)
			if err != nil || retLen <= 0 {
				log.Printf("connect read len=%d, error=%v", retLen, err)
				return
			}
			rcvHandle.HandleFlvData(flvBuff, retLen)

			if totalLen > 1024 {
				break
			}
		}

	}()

	return nil
}

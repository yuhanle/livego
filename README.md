# livego
基于go的rtmp服务器, 主要功能:</br>
常规:</br>
1. rtmp推流, 拉流;</br>
2. http-flv的play服务;</br>
3. http-hls的play服务;</br>
</br>
新增:</br>
1. 提供http动态控制流的rtmp push和pull功能</br>
例子1:</br>
动态pull功能, pull进来一个直播，并在服务器对外生成rtmp://127.0.0.1/live/123456的直播服务</br>
pull开始: </br>
curl -v "http://127.0.0.1:8090/control/pull?oper=start&app=live&name=123456&url=rtmp://live.hkstv.hk.lxdns.com/live/hks"</br>
pull结束: </br>
curl -v "http://127.0.0.1:8090/control/pull?oper=stop&app=live&name=123456&url=rtmp://live.hkstv.hk.lxdns.com/live/hks"</br>

例子2:<br>
动态push功能, 把一个本地已经有的流rtmp://127.0.0.1/live/123456, push到远程rtmp服务器rtmp://alpush.xxxx.cn/live/123456中去</br>
push开始: </br>
curl -v "http://127.0.0.1:8090/control/push?oper=start&app=live&name=123456&url=rtmp://alpush.xxxx.cn/live/123456</br>
push结束: </br>
curl -v "http://127.0.0.1:8090/control/push?oper=stop&app=live&name=123456&url=rtmp://alpush.xxxx.cn/live/123456</br>
</br>
2. 提供静态rtmp push功能</br>
{</br>
    "server": [</br>
	{</br>
	"appname":"live",</br>
	"liveon":"on",</br>
	"hlson":"on",</br>
	"static_push":["rtmp://alpush.xxxx.cn/live", "rtmp://alpush.xxxx.cn/xxxx"]</br>
	}</br>
	]</br>
}</br>
在配置livego.cfg的json中，static_push字段后面的为rtmp push地址列表</br>
配置后，任何push上来的流，都会push音视频到配置的push地址列表相关服务器中
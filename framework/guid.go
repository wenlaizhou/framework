package framework

import (
	"math/rand"
	"time"
	"net"
	"strings"
	"strconv"
	"fmt"
	"os"
)

//COMMON

var ip []int

var ipStr string

var random = rand.New(rand.NewSource(time.Now().UnixNano()))

func init() {

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		os.Stderr.WriteString("Oops:" + err.Error())
	} else {
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					ips := strings.Split(ipnet.IP.String(), ".")
					for ipStr := range ips {
						ipi, _ := strconv.Atoi(ips[ipStr])
						ip = append(ip, ipi)
					}
					break
				}
			}
		}
	}

	ipStr = fmt.Sprintf("%02X%02X%02X%02X",
		ip[0]&0xff, ip[1]&0xff, ip[2]&0xff, ip[3]&0xff)

}

// 32字节uuid生成器
// 8字节ip地址
// 15数字时间戳
// 9随机数
func Guid() string {
	timeStamp := time.Now().Unix()
	return fmt.Sprintf("%s%015X%09X", ipStr,
		timeStamp, random.Uint64()&0xfffffffff)
}

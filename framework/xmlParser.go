package framework

import (
	"regexp"
	"io/ioutil"
	"strings"
	"github.com/wenlaizhou/etree"
)

// read <include src="filePath" /> and replace string
func processInclude(data string) string {

	includeReg := regexp.MustCompile(`<\s*include\s*src\s*=\s*("(.*)"|'(.*)').*/>`)
	allSub := includeReg.FindAllStringSubmatch(string(data), -1)
	if len(allSub) <= 0 {
		return data
	}
	for _, subList := range allSub {
		var filePath string
		if len(subList[2]) > 0 {
			filePath = subList[2]
		} else {
			filePath = subList[3]
		}
		includeData, err := ioutil.ReadFile(filePath)
		if err != nil {
			continue
		}
		data = strings.Replace(data, subList[0],
			strings.TrimSpace(string(includeData)), 1)
	}
	return data
}

func LoadXml(filePath string) *etree.Document {
	doc := etree.NewDocument()
	doc.ReadFromString(processInclude(ReadString(filePath)))
	return doc
}

package main

import (
	//	"fmt"
	"encoding/json"
	updog "github.com/TrilliumIT/updog/types"

	//"github.com/gopherjs/gopherjs/js"
	"github.com/gopherjs/jquery"
	"regexp"
)

const jsonData = `{"hdfs":{"Services":{"datanode":{"Instances":{"http://hdfs-datanode1.dc0.cl.trilliumstaffing.com:50075":{"Up":false,"ResponseTime":16060387,"TimeStamp":"2017-05-16T15:58:51.765713621-04:00"},"http://hdfs-datanode2.dc0.cl.trilliumstaffing.com:50075":{"Up":false,"ResponseTime":15708697,"TimeStamp":"2017-05-16T15:58:51.765828016-04:00"},"http://hdfs-datanode3.dc0.cl.trilliumstaffing.com:50075":{"Up":false,"ResponseTime":19502283,"TimeStamp":"2017-05-16T15:58:51.769685078-04:00"},"http://hdfs-datanode4.dc0.cl.trilliumstaffing.com:50075":{"Up":false,"ResponseTime":15218663,"TimeStamp":"2017-05-16T15:58:51.764579347-04:00"},"http://hdfs-datanode5.dc0.cl.trilliumstaffing.com:50075":{"Up":false,"ResponseTime":16054710,"TimeStamp":"2017-05-16T15:58:51.765564738-04:00"}},"Up":0,"Down":5,"IsDegraded":true,"IsFailed":true},"journalnode":{"Instances":{"http://hdfs-journalnode1.dc0.cl.trilliumstaffing.com:8480":{"Up":true,"ResponseTime":3865341,"TimeStamp":"2017-05-16T15:58:51.754043152-04:00"},"http://hdfs-journalnode2.dc0.cl.trilliumstaffing.com:8480":{"Up":true,"ResponseTime":3628899,"TimeStamp":"2017-05-16T15:58:51.753397661-04:00"},"http://hdfs-journalnode3.dc0.cl.trilliumstaffing.com:8480":{"Up":true,"ResponseTime":3368739,"TimeStamp":"2017-05-16T15:58:51.753141597-04:00"},"http://hdfs-journalnode4.dc0.cl.trilliumstaffing.com:8480":{"Up":true,"ResponseTime":3058325,"TimeStamp":"2017-05-16T15:58:51.752830924-04:00"},"http://hdfs-journalnode5.dc0.cl.trilliumstaffing.com:8480":{"Up":true,"ResponseTime":3323824,"TimeStamp":"2017-05-16T15:58:51.752777344-04:00"}},"Up":5,"Down":0,"IsDegraded":false,"IsFailed":false},"namenode":{"Instances":{"http://hdfs-namenode1.dc0.cl.trilliumstaffing.com:50070":{"Up":true,"ResponseTime":3264026,"TimeStamp":"2017-05-16T15:58:51.752785253-04:00"},"http://hdfs-namenode2.dc0.cl.trilliumstaffing.com:50070":{"Up":true,"ResponseTime":3274958,"TimeStamp":"2017-05-16T15:58:51.752624656-04:00"}},"Up":2,"Down":0,"IsDegraded":false,"IsFailed":false}}},"zookeeper":{"Services":{"client":{"Instances":{"zookeeper1.dc0.cl.trilliumstaffing.com:2181":{"Up":true,"ResponseTime":15046206,"TimeStamp":"2017-05-16T15:58:51.764576418-04:00"},"zookeeper2.dc0.cl.trilliumstaffing.com:2181":{"Up":true,"ResponseTime":14907672,"TimeStamp":"2017-05-16T15:58:51.764569652-04:00"},"zookeeper3.dc0.cl.trilliumstaffing.com:2181":{"Up":true,"ResponseTime":11391557,"TimeStamp":"2017-05-16T15:58:51.760765403-04:00"},"zookeeper4.dc0.cl.trilliumstaffing.com:2181":{"Up":true,"ResponseTime":14491691,"TimeStamp":"2017-05-16T15:58:51.764031955-04:00"},"zookeeper5.dc0.cl.trilliumstaffing.com:2181":{"Up":true,"ResponseTime":12321456,"TimeStamp":"2017-05-16T15:58:51.761775814-04:00"}},"Up":5,"Down":0,"IsDegraded":false,"IsFailed":false}}}}`

var (
	jQuery    = jquery.NewJQuery
	jqIDChars *regexp.Regexp
)

func init() {
}

func main() {
	apps := make(map[string]updog.ApplicationStatus)
	err := json.Unmarshal([]byte(jsonData), &apps)
	if err != nil {
		println(err.Error())
		println("Error unmarshalling json")
		return
	}

	for an, app := range apps {
		addIfMissing("application_"+an, "li", "applications", "ul")
		addWithText("application_"+an+"_name", "div", "application_"+an, "li", an)
		addIfMissing(an+"_services", "ul", "application_"+an, "li")
		for sn, svc := range app.Services {
			addIfMissing("service_"+sn, "li", an+"_services", "ul")
			addWithText("service_"+sn+"_name", "div", "service_"+sn, "li", sn)
			addIfMissing(sn+"_instances", "ul", "service_"+sn, "li")
			for in := range svc.Instances {
				addIfMissing("instance_"+in, "li", sn+"_instances", "ul")
				addWithText("instance_"+in+"_name", "div", "instance_"+in, "li", in)
			}
		}
	}

	println(apps)
	//jQuery("ul#applications").SetText("hello")
	//println(jsonData)
	//js.Global.Get("applications").Call("write", json)
}

func replaceInvalidIDChars(id string) string {
	if jqIDChars == nil {
		jqIDChars = regexp.MustCompile("([" + regexp.QuoteMeta("!\"#$%&'()*+,./:;<=>?@[\\]^`{|}~") + "])")
	}
	return jqIDChars.ReplaceAllString(id, "_")
}

func addWithText(id, element, parentId, parentElement string, content interface{}) {
	addIfMissing(id, element, parentId, parentElement)
	setText(id, element, content)
}

func setText(id, element string, content interface{}) {
	id = replaceInvalidIDChars(id)
	jQuery(element + "#" + id).SetText(content)
}

func addIfMissing(id, element, parentId, parentElement string) {
	parentId = replaceInvalidIDChars(parentId)
	id = replaceInvalidIDChars(id)
	if jQuery(element+"#"+id).Length <= 0 {
		jQuery(parentElement + "#" + parentId).Append("<" + element + " id=\"" + id + "\"></" + element + ">")
	}
}

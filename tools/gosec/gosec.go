package main

import (
	"encoding/xml"
	"flag"
	"io/ioutil"
	"log"
	"sort"
	"time"
)

type junitXMLReport struct {
	XMLName    xml.Name       `xml:"testsuites"`
	Testsuites testsuiteSlice `xml:"testsuite"`
}

type testsuite struct {
	XMLName   xml.Name      `xml:"testsuite"`
	Name      string        `xml:"name,attr"`
	Tests     int           `xml:"tests,attr"`
	Testcases testcaseSlice `xml:"testcase"`
}
type testsuiteSlice []testsuite

func (s testsuiteSlice) Len() int { return len(s) }

func (s testsuiteSlice) Less(i, j int) bool { return s[i].Name > s[j].Name }

func (s testsuiteSlice) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

type testcaseSlice []testcase

func (s testcaseSlice) Len() int { return len(s) }

func (s testcaseSlice) Less(i, j int) bool {
	if s[i].Name == s[j].Name {
		return s[i].Failure.Text > s[j].Failure.Text
	}
	return s[i].Name > s[j].Name
}

func (s testcaseSlice) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

type testcase struct {
	XMLName xml.Name `xml:"testcase"`
	Name    string   `xml:"name,attr"`
	Failure failure  `xml:"failure"`
}

type failure struct {
	XMLName xml.Name `xml:"failure"`
	Message string   `xml:"message,attr"`
	Text    string   `xml:",innerxml"`
}

var (
	junitFile string
)

func init() {
	flag.StringVar(&junitFile, "junit", "junit-gosec.xml", "usage")
}

func parseJunit(filename string) *junitXMLReport {
	log.Printf("Reading data from %s", filename)
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalf("Failed to read report, %v", err)
	}

	var junit junitXMLReport
	xml.Unmarshal(data, &junit)
	return &junit
}

func main() {
	flag.Parse()

	newJunit := parseJunit(junitFile)
	// oldJunit := parseJunit(oldFile)

	slice := newJunit.Testsuites

	sort.SliceStable(slice, func(i, j int) bool {
		return slice[i].Name > slice[j].Name
	})

	// slice = oldJunit.Testsuites

	// sort.SliceStable(slice, func(i, j int) bool {
	// 	return slice[i].Name > slice[j].Name
	// })

	for _, testSuit := range newJunit.Testsuites {
		slice := testSuit.Testcases
		sort.SliceStable(slice, func(i, j int) bool {
			return slice[i].Name > slice[j].Name
		})

	}
	xmlHeader := []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	data, err := xml.MarshalIndent(newJunit, "", "\t")
	if err != nil {
		log.Fatalf("Couldn't marshal data, %v", err)
	}
	// err = os.Remove(junitFile)
	// if err != nil {
	// 	log.Fatalf("Couldn't remove junit file, %v", err)
	// }
	ioutil.WriteFile(junitFile+time.Now().String(), append(xmlHeader, data...), 0644)
	// for _, testSuit := range oldJunit.Testsuites {
	// 	slice := testSuit.Testcases
	// 	sort.SliceStable(slice, func(i, j int) bool {
	// 		return slice[i].Name > slice[j].Name
	// 	})
	// }

	// if !reflect.DeepEqual(*&newJunit.Testsuites, *&oldJunit.Testsuites) {
	// 	log.Fatal("There is a new issue")
	// }
	// report := kubevirtReport{Cases: []goSecCase{}}

	// for _, testsuite := range junit.Testsuites {
	// 	report.Cases = append(report.Cases, goSecCase{testsuite.Name, testsuite.Tests})
	// }

	// data, err = ioutil.ReadFile(reportFile)
	// if err != nil {
	// 	log.Fatalf("Failed to read old report, %v", err)
	// }
	// oldReport := kubevirtReport{Cases: []goSecCase{}}
	// err = json.Unmarshal(data, &oldReport)
	// if err != nil {
	// 	log.Fatalf("Failed to read old report, %v", err)
	// }

}

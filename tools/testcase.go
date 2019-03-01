package tools

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os/exec"
	"regexp"
	"strings"
)

type TestCase struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}

type TestResult struct {
	Pass    bool
	Output  string
	Predict string
}

func fetchTestcase(cn, diff string) ([]TestCase, error) {
	URL := "https://atcoder.jp/contests/" + cn + "/tasks/" + cn + "_" + diff

	fmt.Println(URL)
	resp, err := client.Get(URL)
	if err != nil {
		log.Fatal(err)
		return []TestCase{}, err
	}
	defer resp.Body.Close()

	d, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
		return []TestCase{}, err
	}

	prp := regexp.MustCompile(`<section>\s*<h3>[入出]力例[\s\S]+?</h3>([\s\S]+?)</section>`)
	l := prp.FindAllString(string(d), -1)

	ret := make([]TestCase, len(l)/2)

	rp := regexp.MustCompile(`<pre.*?>([\s\S]+?)</pre>`)
	for i, v := range l {
		r := rp.FindAllStringSubmatch(string(v), -1)
		if i%2 == 0 {
			ret[i/2].Input = r[0][1]
		} else {
			ret[i/2].Output = r[0][1]
		}
	}

	return ret, nil
}

func uniteNewLineCode(s string) string {
	r := strings.NewReplacer("\r\n", "\n", "\r", "\n", "\n", "\n")
	return r.Replace(s)
}

func innerCases(c TestCase, path string) TestResult {
	cmd := exec.Command("go", "run", path)
	res := TestResult{}
	in, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
		return res
	}
	defer in.Close()
	io.WriteString(in, c.Input)

	out, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
		return res
	}

	res.Output = uniteNewLineCode(string(out))
	res.Predict = uniteNewLineCode(c.Output)
	res.Pass = (strings.Compare(res.Output, res.Predict) == 0)

	return res
}

func tryTests(done <-chan interface{}, cases []TestCase, filePath string) <-chan TestResult {
	resStream := make(chan TestResult, len(cases))
	go func() {
		defer close(resStream)
		for _, v := range cases {
			go func(t TestCase) {
				resStream <- innerCases(t, filePath)
			}(v)
		}
		for {
			select {
			case <-done:
				return
			}
		}
	}()

	return resStream
}

func DoTestcase(contestName, difficulty, filePath string) bool {
	cases, _ := fetchTestcase(contestName, difficulty)

	isPassed := true

	done := make(chan interface{})
	results := tryTests(done, cases, filePath)
	fmt.Println("*", len(cases), "cases trying...")
	for i := 1; i <= len(cases); i++ {
		r := <-results
		if r.Pass {
			fmt.Println("* test", i, "passed")
		} else {
			fmt.Println("! test", i, "rejected")
			isPassed = false
		}
		fmt.Println("- output:")
		fmt.Println(r.Output)
		fmt.Println("- Predict:")
		fmt.Println(r.Predict)
	}
	close(done)

	return isPassed
}

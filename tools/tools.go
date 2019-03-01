package tools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/net/html"
)

type UserInfo struct {
	Username string `json:"user_name"`
	Password string `json:"password"`
}

type TestCase struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}

type TestResult struct {
	Pass    bool
	Output  string
	Predict string
}

var client *http.Client
var csrfToken string

func init() {
	jar, err := cookiejar.New(nil)
	if err != nil {
		log.Fatal(err)
	}
	client = &http.Client{
		Jar: jar,
	}
	tryLogin()
}

func tryLogin() {
	sfileName := ".session.json"

	if _, fe := os.Stat(sfileName); os.IsNotExist(fe) {
		if err := fetchSession(sfileName); err != nil {
			log.Fatal(err)
		}

	}

	_, err := os.Open(sfileName)
	if err != nil {
		fmt.Println(err)
	}
}

func inputUserInfo() UserInfo {
	var info UserInfo
	infofile := ".userinfo.json"

	if _, fe := os.Stat(infofile); !os.IsNotExist(fe) {
		d, err := ioutil.ReadFile(infofile)
		if err != nil {
			log.Fatal(err)
		}
		if err := json.Unmarshal(d, &info); err != nil {
			log.Fatal(err)
		}
		fmt.Println("* read:", infofile)
		return info
	} else {
		fmt.Println(fe)
	}

	fmt.Println("* input userinfo")
	sc := bufio.NewScanner(os.Stdin)
	fmt.Print("Username: ")
	sc.Scan()
	info.Username = sc.Text()

	fmt.Print("Password: ")
	pass, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.Fatal(err)
	}
	info.Password = string(pass)
	fmt.Println()

	mar, err := json.Marshal(info)
	if err != nil {
		log.Fatal(err)
	}

	if err := ioutil.WriteFile(infofile, mar, 0600); err != nil {
		log.Fatal(err)
	}
	fmt.Println("* write:", infofile)

	return info
}

func removeUserInfo() {
	infofile := ".userinfo.json"
	if err := os.Remove(infofile); err != nil {
		log.Fatal(err)
	}
	fmt.Println("* delete:", infofile)
}

func fetchSession(n string) error {

	info := inputUserInfo()

	URL := "https://atcoder.jp/login"

	resp, err := client.Get(URL)
	if err != nil {
		log.Fatal(err)
		return err
	}

	ck := resp.Cookies()
	spl := strings.Split(ck[1].String(), " ")
	tidx := strings.Index(spl[0], "csrf_token")
	combtoken, _ := url.QueryUnescape(spl[0][tidx:])
	cidx := strings.Index(combtoken, ":")
	cfin := strings.Index(combtoken, "=")
	csrfToken = combtoken[cidx+1 : cfin+1]

	data := url.Values{}
	data.Add("csrf_token", csrfToken)
	data.Add("password", info.Password)
	data.Add("username", info.Username)

	pres, err := client.PostForm(URL, data)
	if err != nil {
		log.Fatal(err)
		return err
	}

	if n := indexInHtmlTag("title", "Sign", pres.Body); n >= 0 {
		fmt.Println("! login failed")
		removeUserInfo()
		return fmt.Errorf("failed login")
	}
	fmt.Println("* login success")

	defer resp.Body.Close()

	return nil
}

func indexInHtmlTag(tag, ct string, r io.ReadCloser) int {
	tk := html.NewTokenizer(r)

	for {
		tt := tk.Next()
		if tt == html.ErrorToken {
			return -1
		}

		tn, _ := tk.TagName()
		if string(tn) == tag {
			tk.Next()
			fmt.Println(string(tk.Text()))
			return strings.Index((string(tk.Text())), ct)
		}
	}
	return -1
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

func PostAnswer(contestName, difficulty, filePath string) {
	fmt.Println("* post Answer", contestName+"_"+difficulty)

	pdata := url.Values{}
	pdata.Add("data.TaskScreenName", contestName+"_"+difficulty)
	pdata.Add("data.LanguageId", "3013")
	pdata.Add("csrf_token", csrfToken)

	f, err := os.Open(filePath)
	if err != nil {
		fmt.Println("! file cannot open")
		log.Fatal(err)
	}
	defer f.Close()
	fv, _ := ioutil.ReadAll(f)

	pdata.Add("sourceCode", string(fv))

	URL := "https://atcoder.jp/contests/" + contestName + "/submit"

	res, err := client.PostForm(URL, pdata)
	if err != nil {
		fmt.Println("! post answer failed")
		log.Fatal(err)
	}
	defer res.Body.Close()

	fmt.Println("* post answer success")
	fmt.Println("  check submission page")
	fmt.Println(" ", "https://atcoder.jp/contest/"+contestName+"/submissions/me")
}

func TrySolve(contestName, difficulty, filePath string) {
	ispassed := DoTestcase(contestName, difficulty, filePath)

	if !ispassed {
		fmt.Println("! failed test case")
		fmt.Println("  try again")
		return
	}
	fmt.Println("* passed test case")

	PostAnswer(contestName, difficulty, filePath)

}

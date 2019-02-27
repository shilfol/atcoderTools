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

var client *http.Client

func init() {
	jar, err := cookiejar.New(nil)
	if err != nil {
		log.Fatal(err)
	}
	client = &http.Client{
		Jar: jar,
	}
}

func tryLogin() {
	sfileName := ".session.json"

	if _, fe := os.Stat(sfileName); os.IsNotExist(fe) {
		fetchSession(sfileName)
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

func fetchSession(n string) {

	info := inputUserInfo()

	URL := "https://atcoder.jp/login"

	resp, err := client.Get(URL)
	if err != nil {
		log.Fatal(err)
	}

	ck := resp.Cookies()
	spl := strings.Split(ck[1].String(), " ")
	tidx := strings.Index(spl[0], "csrf_token")
	combtoken, _ := url.QueryUnescape(spl[0][tidx:])
	cidx := strings.Index(combtoken, ":")
	cfin := strings.Index(combtoken, "=")
	csrfToken := combtoken[cidx+1 : cfin+1]

	data := url.Values{}
	data.Add("csrf_token", csrfToken)
	data.Add("password", info.Password)
	data.Add("username", info.Username)

	pres, err := client.PostForm(URL, data)
	if err != nil {
		log.Fatal(err)
	}

	if n := indexInHtml("Sign", pres.Body); n < 0 {
		fmt.Println("! login failed")
		removeUserInfo()
	}
	fmt.Println("* login success")

	defer resp.Body.Close()

	/*
		pdata := url.Values{}
		pdata.Add("data.TaskScreenName", "abc118_d")
		pdata.Add("data.LanguageId", "3013")
		pdata.Add("csrf_token", csrfToken)

		f, err := os.Open("./abc/118/d-dp.go")
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		fv, _ := ioutil.ReadAll(f)

		pdata.Add("sourceCode", string(fv))

		SURL := "https://atcoder.jp/contests/abc118/submit"

		sres, err := client.PostForm(SURL, pdata)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(sres)
		sv, _ := ioutil.ReadAll(sres.Body)
		fmt.Println(string(sv))
		defer sres.Body.Close()
	*/

}

func indexInHtml(t string, r io.ReadCloser) int {
	tk := html.NewTokenizer(r)

	for {
		tt := tk.Next()
		if tt == html.ErrorToken {
			return -1
		}

		tn, _ := tk.TagName()
		if string(tn) == "title" {
			tk.Next()
			fmt.Println(string(tk.Text()))
			return strings.Index((string(tk.Text())), t)
		}
	}
	return -1
}

func fetchTestcase(cn string) {
	URL := "https://atcoder.jp/contests/" + cn + "/tasks/" + cn + "_d"

	fmt.Println(URL)
	resp, err := client.Get(URL)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	d, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
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

	fmt.Println(ret)
}
func doTestcase() {

}

func postAnswer() {

}

func TrySolve() {
	//tryLogin()
	fetchTestcase("abc118")
	//doTestcase()

	//postAnswer()

}

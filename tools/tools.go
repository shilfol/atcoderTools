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
	"strings"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/net/html"
)

type UserInfo struct {
	Username string `json:"user_name"`
	Password string `json:"password"`
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
	if err := TryLogin(); err != nil {
		log.Fatal(err)
	}
}

func InputUserInfo() UserInfo {
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

func RemoveUserInfo() {
	infofile := ".userinfo.json"
	if err := os.Remove(infofile); err != nil {
		log.Fatal(err)
	}
	fmt.Println("* delete:", infofile)
}

func TryLogin() error {

	info := InputUserInfo()

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
		RemoveUserInfo()
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
	fmt.Println(" ", "https://atcoder.jp/contests/"+contestName+"/submissions/me")
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

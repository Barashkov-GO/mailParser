package main

import (
	"fmt"
	"github.com/360EntSecGroup-Skylar/excelize"
	"golang.org/x/net/html"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const SearchString = "https://google.ru/search?q="

//const SearchString = "https://nova.rambler.ru/search?utm_source=search&utm_campaign=self_promo&utm_medium=form&utm_content=search&query="
const SearchStringShort = "https://google.ru"
const NextPageLink = "/search?q="
const MailtoString = "mailto:"

var timeOut time.Duration
var contactsString []string // aim-words to find contacts
var banWordsMail []string   // ban-words to find mails
var keyWords []string       // key-words to search in google

var poolUrl []string //	auto-generated pool of url to fd mails

type Mail struct {
	fromUrl string
	mails   map[string]bool
}

func parseUrls(keyWord string) map[string]bool { // find all urls of google search
	doc := openUrl(SearchString + keyWord)
	m := make(map[string]bool)
	if doc == nil {
		return nil
	}
	findUrls(&m, doc)
	return m
}

func parseContacts(url string) string { // find single contacts-link of the input url
	n := openUrl(url)
	if n == nil {
		return ""
	}
	return url + findContacts(n)
}

func parseMails(url string) (Mail, error) { // find all mails of the input url
	var m Mail
	doc := openUrl(url)
	if doc == nil {
		return m, nil
	}
	m.fromUrl = url
	m.mails = make(map[string]bool)
	findMails(&m.mails, doc, MailtoString)
	return m, nil

}

func findUrls(links *map[string]bool, n *html.Node) { //	rec-function to find urls
	if n.Type == html.ElementNode && n.Data == "a" {
		for _, a := range n.Attr {
			if a.Key == "href" && strings.Contains(a.Val, NextPageLink) {
				poolUrl = append(poolUrl, SearchStringShort+a.Val)
			} else {
				if a.Key == "href" {
					(*links)[trashReplace(a.Val)] = true
				}
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		findUrls(links, c)
	}
}

func findContacts(n *html.Node) string { // rec-function to find contacts-part of site
	if n.Type == html.ElementNode && n.Data == "a" {
		for _, a := range n.Attr {
			if a.Key == "href" && checkKeyWords(a.Namespace) {
				fmt.Println(a.Val, a.Namespace)
				return trashReplace(a.Val)
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		findContacts(c)
	}
	//findContacts(n.NextSibling)
	return ""
}

func findMails(links *map[string]bool, n *html.Node, substr string) { //	rec-function to find mails
	if n.Type == html.ElementNode && n.Data == "a" {
		for _, a := range n.Attr {
			if a.Key == "href" && strings.Contains(a.Val, substr) && checkBanWords(a.Val) {
				(*links)[trashReplace(a.Val[len(substr):])] = true
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		findMails(links, c, substr)
	}
}

func openUrl(url string) *html.Node { // opens the input url
	resp, err := http.Get(url)
	if err != nil {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		err := resp.Body.Close()
		if err != nil {
			return nil
		}
	}
	n, err := html.Parse(resp.Body)
	err = resp.Body.Close()
	if err != nil {
		return nil
	}
	return n
}

func trashReplace(str string) string { //	replacing bad part of links
	str = strings.ReplaceAll(str, "/url?q=", "")
	ind := strings.Index(str, "&sa=U&ved=")
	if ind > 0 {
		str = str[:ind]
	}
	return str
}

func checkBanWords(s string) bool { // check mail to not contain ban-words for finding mail
	for _, v := range banWordsMail {
		if strings.Contains(s, v) {
			return false
		}
	}
	return true
}

func checkKeyWords(s string) bool { // check namespace to contain key-words for finding contacts
	for _, v := range contactsString {
		if strings.Contains(s, v) {
			return true
		}
	}
	return false
}

func writeJson(path string, s string) {
	b := []byte(s)
	if len(b) == 0 {
		return
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		os.Exit(1)
	}
	defer file.Close()
	file.Write(b)
}

func fillExcel(sUrl string, sMail string, m *map[string]string) {
	(*m)[sMail] = sUrl
}

func printMap(m map[string]string) {
	for k, v := range m {
		fmt.Println(k, v)
	}
}

func fileInput(path string) { //	parsing input file
	f, e := ioutil.ReadFile(path)
	if e != nil {
		return
	}
	fArr := strings.Split(string(f), "\n")
	contactsString = strings.Split(fArr[0], ",")
	banWordsMail = strings.Split(fArr[1], " ")
	keyWords = strings.Split(fArr[2], ",")
	t, _ := strconv.Atoi(fArr[3])
	timeOut = time.Minute * time.Duration(t)
}

func writeExcel(path string, m map[string]string) { //	fills the excel-document within map
	f := excelize.NewFile()
	ind := 1
	for mail, url := range m {
		s1 := "A" + strconv.Itoa(ind)
		s2 := "B" + strconv.Itoa(ind)
		f.SetCellValue("Sheet1", s1, url)
		f.SetCellValue("Sheet1", s2, mail)
		ind += 1
	}
	err := f.SaveAs(path)
	if err != nil {
		fmt.Println(err)
		return
	}
}

func createPool(keyWords []string) { // creates pool of urls of the key-words searching
	for _, keyWord := range keyWords {
		poolUrl = append(poolUrl, SearchString+strings.ReplaceAll(keyWord, " ", "%20"))
	}
}

func main() {
	start := time.Now()
	fileInput(os.Args[1])
	mapExcel := make(map[string]string)
	createPool(keyWords)
	for len(poolUrl) != 0 && time.Since(start) < timeOut {
		urlFromPool := poolUrl[0]
		poolUrl = poolUrl[1:]
		urls := parseUrls(urlFromPool)
		for url, _ := range urls {
			url = parseContacts(url)
			mail, _ := parseMails(url)
			if url != "" && len(mail.mails) > 0 {
				for m, _ := range mail.mails {
					fillExcel(url, m, &mapExcel)
				}
			}
		}
	}
	writeExcel("results_"+time.Since(start).String()+".xlsx", mapExcel)
}

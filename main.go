package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"golang.org/x/net/html"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// We must export it to allow JSON to marshal it
type Post struct {
	Title    string
	URL      string
	Author   string
	Points   int
	Comments int
	Rank     int
}

type Posts []Post

type comparator func(node *html.Node) bool

func findNode(n *html.Node, compare comparator) []*html.Node {
	matches := make([]*html.Node, 0)
	if n == nil {
		return matches
	}

	if compare(n) {
		matches = append(matches, n)
	}

	matches = append(matches, findNode(n.FirstChild, compare)...)
	matches = append(matches, findNode(n.NextSibling, compare)...)

	return matches
}

func prevSiblingUntil(node *html.Node, compare comparator) *html.Node {
	for node.PrevSibling != nil {
		node = node.PrevSibling
		if node != nil && compare(node) {
			return node
		}
	}
	return nil
}

func hasAttribute(key string, value string, attrs []html.Attribute) bool {
	attr := getAttribute(key, attrs)

	if attr == nil {
		return false
	}

	if attr.Val != value {
		return false
	}

	return true
}

/**
 * Get an attribute
 *
 * If we are doing this a lot, it probably makes sense to create a hash table
 * to look up the attributes, returning look up to on Θ(1) vs Θ(n) with an array
 */
func getAttribute(key string, attrs []html.Attribute) *html.Attribute {
	for _, attr := range attrs {
		if attr.Key == key {
			return &attr
		}
	}

	return nil
}

func findByClass(class string) comparator {
	return func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return false
		}

		return hasAttribute("class", class, n.Attr)
	}
}

func min(x int, y int) int {
	return int(math.Min(float64(x), float64(y)))
}

func getURL(node *html.Node) (string, error) {
	nodes := findNode(node.FirstChild, findByClass("storylink"))
	if len(nodes) != 1 {
		return "", errors.New("uri nodes length is not exactly one")
	}

	node = nodes[0]
	if node.Type != html.ElementNode {
		return "", errors.New("uri node type is not an element node")
	}

	if node.Data != "a" {
		return "", errors.New("uri node is not an anchor")
	}

	href := getAttribute("href", node.Attr)
	if href == nil {
		return "", errors.New("uri node does not have a href attribute")
	}

	u, err := url.Parse(href.Val)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func getTitle(node *html.Node) (string, error) {
	nodes := findNode(node.FirstChild, findByClass("storylink"))
	if len(nodes) != 1 {
		return "", errors.New("author nodes length is not exactly one")
	}

	firstChild := nodes[0].FirstChild
	if firstChild == nil {
		return "", errors.New("author node does not have any children")
	}

	if firstChild.Type != html.TextNode {
		return "", errors.New("author node child is not a text node")
	}

	return firstChild.Data[0:min(len(firstChild.Data), 256)], nil
}

func getAuthor(node *html.Node) (string, error) {
	nodes := findNode(node, findByClass("hnuser"))
	if len(nodes) != 1 {
		return "", errors.New("author nodes length is not exactly one")
	}

	firstChild := nodes[0].FirstChild
	if firstChild == nil {
		return "", errors.New("author node does not have any children")
	}

	if firstChild.Type != html.TextNode {
		return "", errors.New("author node child is not a text node")
	}

	return firstChild.Data[0:min(len(firstChild.Data), 256)], nil
}

func getRank(node *html.Node) (int, error) {
	nodes := findNode(node.FirstChild, findByClass("rank"))
	if len(nodes) != 1 {
		return -1, errors.New("rank nodes length is not exactly one")
	}

	firstChild := nodes[0].FirstChild
	if firstChild == nil {
		return -1, errors.New("rank node does not have any children")
	}

	if firstChild.Type != html.TextNode {
		return -1, errors.New("rank node child is not a text node")
	}

	rank, err := strconv.Atoi(strings.Replace(firstChild.Data, ".", "", 1))
	if err != nil {
		return -1, errors.New("rank failed to convert to integer")
	}

	return rank, nil
}

func getPoints(node *html.Node) (int, error) {
	nodes := findNode(node, findByClass("score"))
	if len(nodes) != 1 {
		return -1, errors.New("point nodes length is not exactly one")
	}

	firstChild := nodes[0].FirstChild
	if firstChild == nil {
		return -1, errors.New("point node does not have any children")
	}

	if firstChild.Type != html.TextNode {
		return -1, errors.New("point node child is not a text node")
	}

	var re = regexp.MustCompile(`\D*points?`)

	points, err := strconv.Atoi(re.ReplaceAllString(firstChild.Data, ""))
	if err != nil {
		return -1, errors.New("point failed to convert to integer")
	}

	return points, nil
}


func isAdvertisement(node *html.Node) (bool, error) {
	textNode, err := getCommentNode(node)
	if err != nil {
		return false, err
	}

	if textNode.Data == "hide" {
		return true, nil
	}

	return false, nil
}

func getCommentNode(node *html.Node) (*html.Node, error) {
	subTextNode := findNode(node, findByClass("subtext"))
	if len(subTextNode) != 1 {
		return nil, errors.New("comment parent nodes length is not exactly one")
	}

	commentNode := prevSiblingUntil(subTextNode[0].LastChild, func(node *html.Node) bool {
		return node.Type == html.ElementNode
	})
	if commentNode == nil {
		return nil, errors.New("comment node is nil")
	}

	textNode := commentNode.FirstChild
	if textNode == nil {
		return nil, errors.New("comment node does not have any children")
	}

	if textNode.Type != html.TextNode {
		return nil, errors.New("comment node child is not a text node")
	}

	return textNode, nil
}

func getComments(node *html.Node) (int, error) {
	textNode, err := getCommentNode(node)
	if err != nil {
		return -1, err
	}

	if textNode.Data == "discuss" {
		return 0, nil
	}

	var re = regexp.MustCompile(`\D*comments?`)

	comments, err := strconv.Atoi(re.ReplaceAllString(textNode.Data, ""))
	if err != nil {
		return -1, errors.New("comments failed to convert to integer")
	}

	return comments, nil
}

func fetch(url string, page int, results chan result, errors chan error) {
	// TODO: Consider sending Accept, Language and User-Agent headers
	// TODO: Ideally we should a url builder here to ensure valid urls are generated
	resp, err := http.Get(url + "?p=" + strconv.Itoa(page))
	if err != nil {
		errors <- err
		return
	}

	node, err := html.Parse(resp.Body)
	if err != nil {
		errors <- err
		return
	}

	posts, err := getPosts(node)
	if err != nil {
		errors <- err
		return
	}

	results <- result{
		page: page,
		posts: posts,
	}
}

type result struct {
	page int
	posts Posts
}

func main() {
	var postsToFetch int
	var newPosts bool

	flags := flag.NewFlagSet("main", flag.ExitOnError)
	flags.IntVar(&postsToFetch, "posts", 30, "How many posts to print. A positive integer <= 100.")
	flags.BoolVar(&newPosts, "new", false, "Whether to fetch posts from newest as opposed to front page (default false)")

	err := flags.Parse(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	if postsToFetch < 0 && postsToFetch > 100 {
		log.Fatalf("%s", "Posts must be between 1 and 100, inclusive.")
	}

	resultChan := make(chan result)
	errorChan := make(chan error)

	postsPerPage := 30
	pagesToFetch := math.Ceil(float64(postsToFetch) / float64(postsPerPage))

	u := "https://news.ycombinator.com/"
	if newPosts {
		u += "newest"
	} else {
		u += "news"
	}

	for page := 1.0; page <= pagesToFetch; page += 1.0 {
		go fetch(u, int(page), resultChan, errorChan)
	}

	pagesFetched := 0.0
	posts := make([]Post, postsToFetch)
Loop:
	for {
		select {
		case result, ok := <-resultChan:
			if !ok {
				continue
			}
			pagesFetched += 1

			offset := (result.page - 1) * postsPerPage
			for index, post := range result.posts {
				if (offset + index) > len(posts) - 1 {
					break
				}
				posts[offset + index] = post
			}

			if int(pagesFetched) == int(pagesToFetch) {
				break Loop
			}
		case err, ok := <-errorChan:
			if !ok {
				continue
			}
			log.Fatal(err)
		default:
			if errorChan == nil && resultChan == nil {
				break Loop
			}
		}
	}

	response, err := json.MarshalIndent(posts[0:postsToFetch], "", "    ")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(response))
}

func getPosts(node *html.Node) (Posts, error) {
	// NOTE: we could make this allocation more efficient by passing in the length and allocating up front
	posts := make(Posts, 0)
	for _, postNode := range findNode(node, findByClass("athing")) {
		title, err := getTitle(postNode)
		if err != nil {
			return nil, err
		}

		u, err := getURL(postNode)
		if err != nil {
			return nil, err
		}

		nextRow := postNode.NextSibling.FirstChild
		// If nextRow is nil, it's likely we're at the end of the results
		if nextRow == nil {
			continue
		}

		author := "N/A"
		points := -1
		comments := -1

		isAd, err := isAdvertisement(nextRow)

		if err != nil {
			return nil, err
		} else if !isAd {
			author, err = getAuthor(nextRow)
			if err != nil {
				return nil, err
			}

			points, err = getPoints(nextRow)
			if err != nil {
				return nil, err
			}

			comments, err = getComments(nextRow)
			if err != nil {
				return nil, err
			}
		}


		rank, err := getRank(postNode)
		if err != nil {
			return nil, err
		}

		post := Post{
			Title:    title,
			URL:      u,
			Author:   author,
			Points:   points,
			Comments: comments,
			Rank:     rank,
		}
		posts = append(posts, post)
	}
	return posts, nil
}

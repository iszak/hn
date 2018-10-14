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
	"os"
	"regexp"
	"strconv"
	"strings"
)

// We must export it to allow JSON to marshal it
type Post struct {
	Title    string
	URI      string
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

func getUri(node *html.Node) (string, error) {
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

	return href.Val, nil
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

	// TODO: Ideally we should have a "innerText" sort of method here.
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

	// TODO: Ideally we should have a "innerText" sort of method here.
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

	// TODO: Ideally we should have a "innerText" sort of method here.
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

	points, err := strconv.Atoi(strings.Replace(firstChild.Data, " points", "", 1))
	if err != nil {
		return -1, errors.New("point failed to convert to integer")
	}

	return points, nil
}

func getComments(node *html.Node) (int, error) {
	subTextNode := findNode(node, findByClass("subtext"))
	if len(subTextNode) != 1 {
		return -1, errors.New("comment parent nodes length is not exactly one")
	}

	commentNode := prevSiblingUntil(subTextNode[0].LastChild, func(node *html.Node) bool {
		return node.Type == html.ElementNode
	})
	if commentNode == nil {
		return -1, errors.New("comment node is nil")
	}

	textNode := commentNode.FirstChild
	if textNode == nil {
		return -1, errors.New("comment node does not have any children")
	}

	if textNode.Type != html.TextNode {
		return -1, errors.New("comment node child is not a text node")
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

func fetch(page int, results chan Posts, errors chan error) {
	// TODO: Consider spoofing user agent
	resp, err := http.Get("https://news.ycombinator.com/news?p=" + string(page))
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

	results <- posts
}

func main() {
	var postsToFetch int

	flags := flag.NewFlagSet("main", flag.ExitOnError)
	flags.IntVar(&postsToFetch, "posts", 30, "How many posts to print. A positive integer <= 100.")

	err := flags.Parse(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	if postsToFetch < 0 && postsToFetch > 100 {
		log.Fatalf("%s", "Posts must be between 1 and 100, inclusive.")
	}

	results := make(chan Posts)
	errors := make(chan error)

	pagesToFetch := math.Ceil(float64(postsToFetch) / 30.0)
	for page := 1.0; page <= pagesToFetch; page += 1.0 {
		go fetch(int(page), results, errors)
	}

	posts := make(Posts, 0)
Loop:
	for {
		select {
		case result, ok := <-results:
			if !ok {
				continue
			}
			// TODO: Could insert into position (optimal) or sort after the fact
			posts = append(posts, result...)
			if len(posts) >= postsToFetch {
				break Loop
			}
		case err, ok := <-errors:
			if !ok {
				continue
			}
			log.Fatal(err)
		default:
			if errors == nil && results == nil {
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

		uri, err := getUri(postNode)
		if err != nil {
			return nil, err
		}

		nextRow := postNode.NextSibling.FirstChild
		// If nextRow is nil, it's likely we're at the end of the results
		if nextRow == nil {
			continue
		}

		// TODO: Handle ads, with no author
		author, err := getAuthor(postNode)
		if err != nil {
			//return nil, err
		}

		points, err := getPoints(nextRow)
		if err != nil {
			//return nil, err
		}

		comments, err := getComments(nextRow)
		if err != nil {
			//return nil, err
		}

		rank, err := getRank(postNode)
		if err != nil {
			//return nil, err
		}

		post := Post{
			Title:    title,
			URI:      uri,
			Author:   author,
			Points:   points,
			Comments: comments,
			Rank:     rank,
		}
		posts = append(posts, post)
	}
	return posts, nil
}

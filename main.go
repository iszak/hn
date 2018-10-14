package main

import (
	"encoding/json"
	"errors"
	"flag"
	"golang.org/x/net/html"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// We must export it to allow JSON to marshal it
type Post struct {
	Title string
	URI string
	Author string
	Points int
	Comments int
	Rank int
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

func getUri(nodes []*html.Node) (string, error) {
	if len(nodes) != 1 {
		return "", errors.New("uri nodes length is not exactly one")
	}

	node := nodes[0]
	if node.Type != html.ElementNode {
		return "", errors.New("uri node type is not element")
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

func getTitle(nodes []*html.Node) (string, error) {
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

	return firstChild.Data, nil
}

func getAuthor(nodes []*html.Node) (string, error) {
	if len(nodes) != 1 {
		return "", errors.New("author nodes length is not exactly one")
	}

	firstChild := nodes[0].FirstChild
	if firstChild == nil {
		return "", errors.New("title node does not have any children")
	}

	// TODO: Ideally we should have a "innerText" sort of method here.
	if firstChild.Type != html.TextNode {
		return "", errors.New("title node child is not a text node")
	}

	return firstChild.Data, nil
}

func getPoints(nodes []*html.Node) (int, error) {
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
	if node == nil {
		return -1, errors.New("comment node is nil")
	}

	textNode := node.FirstChild
	if node == nil {
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

	// TODO: Parallelise this with channels
	resp, err := http.Get("https://news.ycombinator.com/news?p=1")
	if err != nil {
		log.Fatal(err)
	}

	node, err := html.Parse(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	posts, err := getPosts(node)
	if err != nil {
		log.Fatal(err)
	}

	response, err := json.MarshalIndent(posts, "", "    ")
	if err != nil {
		log.Fatal(err)
	}

	// TODO: Verify this is sent to stdout, may want to use os.StdOut for good measure.
	fmt.Println(string(response))
}

func getPosts(node *html.Node) (Posts, error) {
	// NOTE: we could make this allocation more efficient by passing in the length and allocating up front
	posts := make(Posts, 0)
	for _, postNode := range findNode(node, findByClass("athing")) {
		titleNodes := findNode(postNode.FirstChild, findByClass("storylink"))

		title, err := getTitle(titleNodes)
		if err != nil {
			return nil, err
		}

		uri, err := getUri(titleNodes)
		if err != nil {
			return nil, err
		}

		nextRow := postNode.NextSibling.FirstChild
		if nextRow == nil {
			continue
		}

		// TODO: Handle ads, with no author
		author, err := getAuthor(findNode(nextRow, findByClass("hnuser")))
		if err != nil {
			//return nil, err
		}

		points, err := getPoints(findNode(nextRow, findByClass("score")))
		if err != nil {
			//return nil, err
		}

		subText := findNode(nextRow, findByClass("subtext"))
		// TODO: Check if subtext is nil

		// We may be able to just do subText.LastChild
		commentNode := prevSiblingUntil(subText[0].LastChild, func(node *html.Node) bool {
			return node.Type == html.ElementNode
		})
		comments, err := getComments(commentNode)
		if err != nil {
			//return nil, err
		}
		// TODO: Implement rank

		post := Post{
			Title:  title,
			URI:    uri,
			Author: author,
			Points: points,
			Comments: comments,
		}
		posts = append(posts, post)
	}
	return posts, nil
}
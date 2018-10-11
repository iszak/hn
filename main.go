package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"golang.org/x/net/html"
	"log"
	"net/http"
	"os"
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

	if compare(n) {
		matches = append(matches, n)
	}

	firstChild := n.FirstChild
	if firstChild != nil {
		matches = append(matches, findNode(firstChild, compare)...)
	}

	nextSibling := n.NextSibling
	for nextSibling != nil {
		matches = append(matches, findNode(nextSibling, compare)...)
		nextSibling = nextSibling.NextSibling
	}

	return matches
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

func findPosts(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}

	return hasAttribute("class", "athing", n.Attr)
}

func findTitle(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}

	return hasAttribute("class", "storylink", n.Attr)
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
		return "", errors.New("title nodes length is not exactly one")
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

func main()  {
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
	for _, postNode := range findNode(node, findPosts) {
		// NOTE: we could optimize below to avoid doing 2 tree searches
		title, err := getTitle(findNode(postNode.FirstChild, findTitle))
		uri, err := getUri(findNode(postNode.FirstChild, findTitle))
		// TODO: Implement author, points, comments count and rank

		if err != nil {
			return nil, err
		}

		post := Post{
			Title: title,
			URI: uri,
		}
		posts = append(posts, post)
	}
	return posts, nil
}
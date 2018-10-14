# READ ME

## How to run it

Install docker (latest)

Start docker

Build it

    docker build --tag=hn .

Run it

    docker run hn -help
    docker run hn -posts=1

## Language and Libraries
Go was chosen for a few reasons;

- Go is a compiled language. This allows us to produce a cross platform binaries with all dependencies which can easily be run. Compared to an interpreted language, which requires a runtime to be bundled with or already installed. by bundling the interpreter into an executable.
- Go is a simple language, compared to Ruby, in terms of number of constructs and syntax.
- Go has built in constructs for concurrency. As we are requesting pages, and we know the format for, we can request multiple pages in parallel.
- Go has an "experimental" support for parsing HTML.


## Testing
Although no tests are written, most of the code that selects each struct field is easily testable with some HTML
fixtures and black box testing. The testing of the fetch method may require injecting a function which fetches the
page, to allow mocking the response.

## Suggestions
- There is some duplication on checking for child nodes, node types, etc. Function composition could be used to chain errors together.
- Extract out DOM tree manipulation functions, e.g. prevSiblingsUntil, hasAttributeKey, findNode into separate package. These could easily be used in other applications. Personally, I prefer using CSS selectors into of manually selecting children.
- Better indication if a post is an advertisement instead of checking for -1 in points/comments
- Redirect errors to stderr instead of stdout

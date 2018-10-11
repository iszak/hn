Hacker News Scraper Test

At TrueLayer we love to stay up to date with what is hot in technology and we read Hacker News every day.

In order to make it more fun to read for robots (and easier to integrate in our workflow) we would like to write a simple command line application that would output to STDOUT the top posts in JSON. 

Output format
[
    {
        "title": "Web Scraping in 2016",
        "uri": "https://franciskim.co/2016/08/24/dont-need-no-stinking-api-web-scraping-2016-beyond/",
        "author": "franciskim",
        "points": 133,
        "comments": 80,
        "rank": 1
    },
    {
        "title": "Instapaper is joining Pinterest",
        "uri": "http://blog.instapaper.com/post/149374303661",
        "author": "ropiku",
        "points": 182,
        "comments": 99,
        "rank": 2
    }
]


Input arguments
We expect the application to take these arguments:
hackernews --posts n
--posts how many posts to print. A positive integer <= 100.

Please also ensure that:
title and author are non empty strings not longer than 256 characters.
uri is a valid URI
points, comments and rank are integers >= 0.

Feel free to use any language framework and library you want. Make it concise, readable and correct.

Please describe in README.md:
how to run it (don't assume anything already installed)
what libraries you used and why

Bonus points for:
unit tests
robust input checking
good comments
Dockerfile

Have fun, take your time and when you are done please send a link to your public Github repo at recruiting@truelayer.com


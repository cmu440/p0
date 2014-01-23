p0
==

This repository contains the starter code that you will use as the basis of your multi-client
echo server implementation. It also contains the tests that we will use to test your implementation,
and an example 'server runner' binary that you might find useful for your own testing purposes.

If at any point you have any trouble with building, installing, or testing your code, the article
titled [How to Write Go Code](http://golang.org/doc/code.html) is a great resource for understanding
how Go workspaces are built and organized. You might also find the documentation for the
[`go` command](http://golang.org/cmd/go/) to be helpful. As always, feel free to post your questions
on Piazza as well.

## Running the official tests

To test your submission, we will execute the following command from inside the
`src/github.com/cmu440/p0` directory:

```
$ go test -race
```

This command will run all unit tests inside the `server_test.go` file with the race detector enabled.

To execute a single unit test, you can use the `-test.run` flag and specify a regular expression
identifying the name of the test to run. For example,

```
$ go test -race -test.run TestBasic1
```

## Testing your implementation using `srunner`

To make testing your server a bit easier (especially during the early stages of your implementation
when your server is largely incomplete), we have given you a simple `srunner` (server runner)
program that you can use to create and start an instance of your `MultiEchoServer`. The program
simply creates an instance of your server, starts it on a default port, and blocks forever,
running your server in the background.

To compile and build the `srunner` program into a binary that you can run, execute the three
commands below:

```
$ export GOPATH=$HOME/p0
$ go install github.com/cmu440/srunner
$ $GOPATH/bin/srunner
```

The `srunner` program won't be of much use to you without any clients. It might be a good exercise
to implement your own `crunner` (client runner) program that you can use to connect with and send
messages to your server. We have provided you with an unimplemented `crunner` program that you may
use for this purpose if you wish. Whether or not you decide to implement a `crunner` program will not
affect your grade for this project.

## Using Go on AFS

For those students who wish to write their Go code on AFS (either in a cluster or remotely), you should
beware of the following:

1. As of January 23, 2014, the current version of Go installed on AFS is `v1.0.2`, which is outdated.
   We have submitted a request to install a more recent version of Go on AFS, and we'll let you know
   once that request goes through. Until then, we recommend installing Go on your own computer and
   writing your Go code locally instead.

2. Go is installed to a custom location on AFS machines, so you'll also need to set the `GOROOT`
   environment variable if you are working on a cluster machine or have ssh-ed into one remotely:

    ```
    $ export GOROOT=/usr/local/lib/go
    ```

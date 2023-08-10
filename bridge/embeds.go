package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/companyzero/bisonrelay/zkidentity"
)

type embeddedArgs struct {
	// embedded file
	name string
	data []byte
	alt  string
	typ  string

	// shared link
	download zkidentity.ShortID
	filename string
	size     uint64
	cost     uint64
}

func (args embeddedArgs) String() string {
	var parts []string
	if args.name != "" {
		parts = append(parts, "part="+args.name)
	}
	if args.alt != "" {
		parts = append(parts, "alt="+args.alt)
	}
	if args.typ != "" {
		parts = append(parts, "type="+args.typ)
	}
	if !args.download.IsEmpty() {
		parts = append(parts, "download="+args.download.String())
	}
	if args.filename != "" {
		parts = append(parts, "filename="+args.filename)
	}
	if args.size > 0 {
		parts = append(parts, "size="+strconv.FormatUint(args.size, 10))
	}
	if args.cost > 0 {
		parts = append(parts, "cost="+strconv.FormatUint(args.cost, 10))
	}
	if args.data != nil {
		parts = append(parts, "data="+base64.StdEncoding.EncodeToString(args.data))
	}

	return "--embed[" + strings.Join(parts, ",") + "]--"
}

var embedRegexp = regexp.MustCompile(`--embed\[.*?\]--`)

func parseEmbedArgs(rawEmbedStr string) embeddedArgs {
	// Copy everything between the [] (the raw argument list).
	start, end := strings.Index(rawEmbedStr, "["), strings.LastIndex(rawEmbedStr, "]")
	rawArgs := rawEmbedStr[start+1 : end]

	// Split args by comma.
	splitArgs := strings.Split(rawArgs, ",")

	// Decode args.
	var args embeddedArgs
	for _, a := range splitArgs {
		// Split by "="
		kv := strings.SplitN(a, "=", 2)
		if len(kv) != 2 {
			continue
		}
		k, v := kv[0], kv[1]
		switch k {
		case "name":
			args.name = v
		case "type":
			args.typ = v
		case "data":
			decoded, err := base64.StdEncoding.DecodeString(v)
			if err != nil {
				decoded = []byte(fmt.Sprintf("[err decoding data: %v]", err))
			}
			args.data = decoded
		case "alt":
			decoded, err := url.PathUnescape(v)
			if err != nil {
				decoded = fmt.Sprintf("[err processing alt: %v]", err)
			}
			args.alt = decoded
		case "download":
			// Ignore the error and leave download empty.
			args.download.FromString(v)
		case "filename":
			args.filename = v
		case "size":
			args.size, _ = strconv.ParseUint(v, 10, 64)
		case "cost":
			args.cost, _ = strconv.ParseUint(v, 10, 64)
		}
	}

	return args
}

// replaceEmbeds replaces all the embeds tags of the given text with the result
// of the calling function.
func replaceEmbeds(src string, replF func(args embeddedArgs) string) string {
	return embedRegexp.ReplaceAllStringFunc(src, func(repl string) string {
		// Decode args.
		args := parseEmbedArgs(repl)
		return replF(args)
	})
}

// randomString returns a random, base64-based string.
func randomString() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}

	return base64.URLEncoding.EncodeToString(b[:])
}

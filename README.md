mtgprice
========

Bulk fetch and cache prices for MTG cards/decks

Card data version 2.11.3 from [mtgjson.com](http://mtgjson.com), slightly modified (reformatted).
Pretty-printed using [github.com/broady/json_prettyprint](https://github.com/broady/json_prettyprint)

Notes:

  * No proper error handling in the UI. Some cards may not show up.
  * Need to add retries for the HTTP calls to pricing and Gatherer.

```
$ go get github.com/broady/mtgprice
$ mkdir ~/.mtg && cd ~/.mtg
$ mtgprice
```

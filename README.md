# go-stalk-users
gets tweet, seted users each 5sec.

# Usage
```
$ git clone <this_repository>
$ mkdir images
$ mv config_sample.toml config.toml
// 投稿先とTwitterAPI Keysを書き換え
$ vim config.toml
$ go build
$ nohup ./go-stalk-users -t 'user1 user2 user3' &
```

# Author
[@_numbP](https://twitter.com/_numbP)
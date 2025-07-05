package main

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"crypto/tls"

	"github.com/gin-gonic/gin"
	"github.com/go-pg/pg/v10"
	"github.com/redis/go-redis/v9"
	"gopkg.in/yaml.v3"
)

var dbSession *pg.DB = nil

type Video struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

func getDB(c *gin.Context) *pg.DB {
	if dbSession != nil {
		return dbSession
	}
	endpoint := os.Getenv("DB_ENDPOINT")
	if len(endpoint) == 0 {
		slog.Error("Environment variable `DB_ENDPOINT` is empty")
		c.String(http.StatusBadRequest, "Environment variable `DB_ENDPOINT` is empty")
		return nil
	}
	port := os.Getenv("DB_PORT")
	if len(port) == 0 {
		slog.Error("Environment variable `DB_PORT` is empty")
		c.String(http.StatusBadRequest, "Environment variable `DB_PORT` is empty")
		return nil
	}
	user := os.Getenv("DB_USER")
	if len(user) == 0 {
		user = os.Getenv("DB_USERNAME")
		if len(user) == 0 {
			slog.Error("Environment variables `DB_USER` and `DB_USERNAME` are empty")
			c.String(http.StatusBadRequest, "Environment variables `DB_USER` and `DB_USERNAME` are empty")
			return nil
		}
	}
	pass := os.Getenv("DB_PASS")
	if len(pass) == 0 {
		pass = os.Getenv("DB_PASSWORD")
		if len(pass) == 0 {
			slog.Error("Environment variables `DB_PASS` and `DB_PASSWORD are empty")
			c.String(http.StatusBadRequest, "Environment variables `DB_PASS` and `DB_PASSWORD are empty")
			return nil
		}
	}
	name := os.Getenv("DB_NAME")
	if len(name) == 0 {
		slog.Error("Environment variable `DB_NAME` is empty")
		c.String(http.StatusBadRequest, "Environment variable `DB_NAME` is empty")
		return nil
	}
	dbSession := pg.Connect(&pg.Options{
		Addr:     endpoint + ":" + port,
		User:     user,
		Password: pass,
		Database: name,
	})
	return dbSession
}

func videosGetHandler(ctx *gin.Context) {
	slog.Debug("Handling request", "URI", ctx.Request.RequestURI)
	var videos []Video
	client, err := getRedis()
	if err != nil {
		slog.Error("Error getting redis client", "error", err)
		httpErrorInternalServerError(err, ctx)
		return
	}
	videoCacheKay := "videos"
	val, err := client.Get(ctx, videoCacheKay).Result()
	if err == redis.Nil {
		slog.Warn("key %s does not exist", videoCacheKay)
	} else if err != nil {
		slog.Error("Error fetching from Redis", "error", err)
		httpErrorInternalServerError(err, ctx)
		return
	} else {
		// Deserialize JSON from Redis
		err = json.Unmarshal([]byte(val), &videos)
		if err == nil {
			slog.Warn("Fetched videos from Redis", "count", len(videos))
		}
		slog.Warn("Failed to unmarshal Redis data", "error", err)
	}
	if strings.ToLower(os.Getenv("DB")) == "fs" {
		var err error
		videos, err = getVideosFromFile()
		if err != nil {
			httpErrorInternalServerError(err, ctx)
			return
		}
	} else {
		db := getDB(ctx)
		if db == nil {
			return
		}
		err := db.ModelContext(ctx, &videos).Select()
		if err != nil {
			httpErrorInternalServerError(err, ctx)
			return
		}
	}
	videoJSON, err := json.Marshal(videos)
	if err != nil {
		slog.Error("Failed to marshal videos to JSON", "error", err)
		httpErrorInternalServerError(err, ctx)
		return
	}
	err = client.Set(ctx, videoCacheKay, videoJSON, 0).Err()
	if err != nil {
		slog.Error(fmt.Sprintf("unable store data in redis - %s", err))
	}
	ctx.JSON(http.StatusOK, videos)
}

func getVideosFromFile() ([]Video, error) {
	dir := os.Getenv("FS_DIR")
	if len(dir) == 0 {
		dir = "/cache"
	}
	path := fmt.Sprintf("%s/videos.yaml", dir)
	var videos []Video
	yamlData, err := os.ReadFile(path)
	if err != nil {
		return videos, err
	}
	err = yaml.Unmarshal(yamlData, &videos)

	log.Println("processed get videos request")
	return videos, err
}

func videoPostHandler(ctx *gin.Context) {
	slog.Debug("Handling request", "URI", ctx.Request.RequestURI)
	id := ctx.Query("id")
	if len(id) == 0 {
		//httpErrorBadRequest(errors.New("id is empty"), ctx)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal server error",
		})
		return
	}
	title := ctx.Query("title")
	if len(title) == 0 {
		//httpErrorBadRequest(errors.New("title is empty"), ctx)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal server error",
		})
		return
	}
	video := &Video{
		ID:    id,
		Title: title,
	}
	if strings.ToLower(os.Getenv("DB")) == "fs" {
		videos, err := getVideosFromFile()
		videos = append(videos, *video)
		dir := os.Getenv("FS_DIR")
		if len(dir) == 0 {
			dir = "/cache"
		}
		path := fmt.Sprintf("%s/videos.yaml", dir)
		yamlData, err := yaml.Marshal(videos)
		if err != nil {
			httpErrorInternalServerError(err, ctx)
			return
		}
		err = os.WriteFile(path, yamlData, 0644)
		if err != nil {
			httpErrorInternalServerError(err, ctx)
		}
	} else {
		db := getDB(ctx)
		if db == nil {
			return
		}
		_, err := db.ModelContext(ctx, video).Insert()
		if err != nil {
			httpErrorInternalServerError(err, ctx)
			return
		}
	}

	log.Println("processed post video request")
}

func getRedis() (*redis.Client, error) {

	endpoint := os.Getenv("REDIS_ENDPOINT")
	if len(endpoint) == 0 {
		return nil, fmt.Errorf("Environment variable `REDIS_ENDPOINT` is empty")
	}

	port := os.Getenv("REDIS_PORT")
	if len(port) == 0 {
		return nil, fmt.Errorf("Environment variable `REDIS_PORT` is empty")
	}

	return redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", endpoint, port),
		Password: "", // no password set
		DB:       0,  // use default DB
		TLSConfig: &tls.Config{InsecureSkipVerify: true},
	}), nil
}

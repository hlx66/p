package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"syscall"

	"github.com/labstack/echo"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var db *mongo.Database = nil

func main() {

	servicePtr := flag.Bool("service", false, "Start the server. Default port 8080.")
	portPtr := flag.String("p", "8080", "Port. Default is 8080.")

	flag.Parse()

	db = InitiateMongoDB()

	if *servicePtr {
		service(*portPtr)
		os.Exit(0)
	}
	up(os.Args[1:])
}
func InitiateMongoDB() *mongo.Database {
	uri := "mongodb://p:p%2F7273@ishmael:27017/?authSource=admin"
	opts := options.Client()
	opts.ApplyURI(uri)
	opts.SetMaxPoolSize(5)
	client, err := mongo.Connect(context.Background(), opts)
	if err != nil {
		fmt.Println(err.Error())
	}
	return client.Database("p")
}
func up(files []string) {
	for _, f := range files {
		fi, err := os.Stat(f)
		if err != nil {
			fmt.Println(err)
			continue
		}
		switch mode := fi.Mode(); {
		case mode.IsDir():
			uploadFiles(f)
		case mode.IsRegular():
			uploadFile(f, fi.Name(), fi.Size())
		}
	}
}
func uploadFile(file, filename string, size int64) {
	fmt.Printf("-> %s\n", filename)
	data, err := os.Open(file)
	bucket, err := gridfs.NewBucket(
		db,
	)
	if err != nil {
		fmt.Printf("%s not found", file)
		return
	}
	defer data.Close()
	fmt.Printf("...")
	id, err := bucket.UploadFromStream(filename, data)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	os.Remove(file)
	fmt.Printf("SUCCESS ID: %s Size: %s\n", id.Hex(), byteCountBinary(size))
}
func byteCountBinary(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
func uploadFiles(dirname string) {
	fmt.Println("--> %s", dirname)
	err := filepath.Walk(dirname,
		func(path string, info os.FileInfo, err error) error {
			re := regexp.MustCompile(".jpeg|.jpg|.mp4|.avi|.mp3|.m4v|.wmv|.mpg|.mpeg")
			if err != nil {
				return err
			}
			if re.MatchString(path) == false {
				return nil
			}
			uploadFile(path, info.Name(), info.Size())
			return nil
		})
	if err != nil {
		log.Println(err)
	}
}
func service(port string) {
	httpServer := echo.New()
	httpServer.GET("/file/:id", stream)

	go func() {
		httpServer.Logger.Fatal(httpServer.Start(":" + port))
	}()

	/*
	 * Setup shutdown channel so we can wait for CTRL+C to shut
	 * down the HTTP server
	 */
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	//ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	//defer cancel()
}
func stream(ctx echo.Context) error {
	id, err := primitive.ObjectIDFromHex(ctx.Param("id"))
	fmt.Println("-> Download: %", id.Hex())
	if err != nil {
		fmt.Printf("Invalid ID: %s\n", ctx.Param("id"))
		return ctx.String(http.StatusInternalServerError, "Invalid ID")
	}
	bucket, _ := gridfs.NewBucket(
		db,
	)
	gridFile, err := bucket.OpenDownloadStream(id)
	if err != nil {
		fmt.Printf("File not found with id: %s\n%s\n", id, err)
		return ctx.String(http.StatusInternalServerError, "File not found")

	}
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	//fmt.Println(gridFile)
	//ctx.Response().Header().Set("Content-Length",
	//ctx.Response().Header().Set("Content-Disposition", "inline; filename="+gridFile)

	return ctx.Stream(http.StatusOK, mime.TypeByExtension(".jpg"), gridFile)
}

package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"net"
	"strings"
	"os"
	"path/filepath"
	"net/url"
	"strconv"

	_ "modernc.org/sqlite"
)

func handleClient(conn net.Conn, db *sql.DB) {
	defer conn.Close()

	reader := bufio.NewReader(conn)

	requestLine, err := reader.ReadString('\n')
	if err != nil {
		return
	}

	parts := strings.Fields(requestLine)
	if len(parts) < 3 {
		return
	}

	path := parts[1]

	for {
		line, err := reader.ReadString('\n')
		if err != nil || line == "\r\n" {
			break
		}
	}

	route := path
	page := 1
	sortColumn := ""

	if strings.Contains(path, "?") {
		parts := strings.SplitN(path, "?", 2)
		route = parts[0]

		params, _ := url.ParseQuery(parts[1])

		pageStr := params.Get("page")
		if pageStr != "" {
			p, err := strconv.Atoi(pageStr)
			if err == nil && p > 0 {
				page = p
			}
		}

		sortColumn = params.Get("sort")
	}

	limit := 5
	offset := (page - 1) * limit

	// Servir archivos estáticos
	if strings.HasPrefix(path, "/static/") || path == "/favicon.ico" {

		filePath := "." + path

		data, err := os.ReadFile(filePath)
		if err != nil {
			body := "File not found"
			response :=
				"HTTP/1.1 404 Not Found\r\n" +
					"Content-Type: text/plain\r\n" +
					fmt.Sprintf("Content-Length: %d\r\n", len(body)) +
					"\r\n" +
					body

			conn.Write([]byte(response))
			return
		}

		ext := filepath.Ext(filePath)

		contentType := "text/plain"
		if ext == ".css" {
			contentType = "text/css"
		} else if ext == ".png" {
			contentType = "image/png"
		} else if ext == ".jpg" || ext == ".jpeg" {
			contentType = "image/jpeg"
		} else if ext == ".ico" {
			contentType = "image/x-icon"
		}

		response :=
			"HTTP/1.1 200 OK\r\n" +
				"Content-Type: " + contentType + "\r\n" +
				fmt.Sprintf("Content-Length: %d\r\n", len(data)) +
				"\r\n"

		conn.Write([]byte(response))
		conn.Write(data)
		return
	}

	if route != "/" {
		body := "<html><body><h1>404 Not Found</h1></body></html>"
		response :=
			"HTTP/1.1 404 Not Found\r\n" +
				"Content-Type: text/html\r\n" +
				fmt.Sprintf("Content-Length: %d\r\n", len(body)) +
				"\r\n" +
				body

		conn.Write([]byte(response))
		return
	}

	// ORDER BY seguro
	orderBy := ""

	if sortColumn == "name" {
		orderBy = " ORDER BY name"
	} else if sortColumn == "current" {
		orderBy = " ORDER BY current_episode"
	} else if sortColumn == "total" {
		orderBy = " ORDER BY total_episodes"
	}

	query := "SELECT id, name, current_episode, total_episodes FROM series" + orderBy + " LIMIT ? OFFSET ?"

	rows, err := db.Query(query, limit, offset)
	if err != nil {
		return
	}
	defer rows.Close()

	html := "<html><head><title>Series</title><link rel='stylesheet' href='/static/style.css'></head><body>"

	html += "<div class='pagination'>"

	if page > 1 {
		html += fmt.Sprintf("<a href='/?page=%d'>Previous</a>", page-1)
	}

	html += fmt.Sprintf("<a href='/?page=%d'>Next</a>", page+1)

	html += "</div>"

	html += "<h1>Series que he visto (o estoy viendo)</h1>"
	html += "<table>"
	html += "<tr>"
	html += "<th>ID</th>"
	html += "<th><a href='/?sort=name'>Name</a></th>"
	html += "<th><a href='/?sort=current'>Progress</a></th>"
	html += "</tr>"

	for rows.Next() {
		var id int
		var name string
		var current int
		var total int

		rows.Scan(&id, &name, &current, &total)

		progress := fmt.Sprintf("%d / %d", current, total)

		html += fmt.Sprintf(
			"<tr><td>%d</td><td>%s</td><td>%s</td></tr>",
			id, name, progress,
		)
	}

	html += "</table>"
	html += "</body></html>"

	response :=
		"HTTP/1.1 200 OK\r\n" +
			"Content-Type: text/html\r\n" +
			fmt.Sprintf("Content-Length: %d\r\n", len(html)) +
			"\r\n" +
			html

	conn.Write([]byte(response))
}

func main() {
	db, err := sql.Open("sqlite", "file:series.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	log.Println("Listening on port 8080")

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go handleClient(conn, db)
	}
}
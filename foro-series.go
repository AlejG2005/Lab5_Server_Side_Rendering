package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

func handleClient(conn net.Conn, db *sql.DB) {
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Leer request line
	requestLine, err := reader.ReadString('\n')
	if err != nil {
		return
	}

	parts := strings.Fields(requestLine)
	if len(parts) < 3 {
		return
	}

	method := parts[0]
	path := parts[1]

	// Leer headers y capturar Content-Length
	contentLength := 0

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		if strings.HasPrefix(line, "Content-Length:") {
			lengthStr := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			contentLength, _ = strconv.Atoi(lengthStr)
		}

		if line == "\r\n" {
			break
		}
	}

	route := path
	page := 1
	sortColumn := ""

	// Parsear query params
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

	// SERVIR ESTÁTICOS
	if strings.HasPrefix(path, "/static/") || path == "/favicon.ico" {

		filePath := "." + path
		data, err := os.ReadFile(filePath)
		if err != nil {
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

	// GET /create
	if route == "/create" && method == "GET" {

		html := "<html><head><title>Create Series</title></head><body>"
		html += "<h1>Add New Series</h1>"
		html += "<form method='POST' action='/create'>"
		html += "Name: <input type='text' name='series_name' required><br><br>"
		html += "Current Episode: <input type='number' name='current_episode' min='1' value='1' required><br><br>"
		html += "Total Episodes: <input type='number' name='total_episodes' min='1' required><br><br>"
		html += "<button type='submit'>Add</button>"
		html += "</form>"
		html += "<br><a href='/'>Back</a>"
		html += "</body></html>"

		response :=
			"HTTP/1.1 200 OK\r\n" +
				"Content-Type: text/html\r\n" +
				fmt.Sprintf("Content-Length: %d\r\n", len(html)) +
				"\r\n" +
				html

		conn.Write([]byte(response))
		return
	}

	// POST /create
	if route == "/create" && method == "POST" {

		bodyBytes := make([]byte, contentLength)
		_, err := reader.Read(bodyBytes)
		if err != nil {
			return
		}

		body := string(bodyBytes)

		values, _ := url.ParseQuery(body)

		name := values.Get("series_name")
		current := values.Get("current_episode")
		total := values.Get("total_episodes")

		db.Exec(
			"INSERT INTO series (name, current_episode, total_episodes) VALUES (?, ?, ?)",
			name, current, total,
		)

		response :=
			"HTTP/1.1 303 See Other\r\n" +
				"Location: /\r\n\r\n"

		conn.Write([]byte(response))
		return
	}

	// POST /update
	if route == "/update" && method == "POST" {

		parts := strings.SplitN(path, "?", 2)
		if len(parts) > 1 {
			params, _ := url.ParseQuery(parts[1])
			id := params.Get("id")

			db.Exec(
				"UPDATE series SET current_episode = current_episode + 1 WHERE id = ? AND current_episode < total_episodes",
				id,
			)
		}

		response :=
			"HTTP/1.1 200 OK\r\n" +
				"Content-Type: text/plain\r\n\r\nok"

		conn.Write([]byte(response))
		return
	}

	// 404
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

	// ORDER BY
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
	html += "<a href='/create'>Add New Series</a><br><br>"
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
			"<tr><td>%d</td><td>%s</td><td>%s <button onclick='nextEpisode(%d)'>+1</button></td></tr>",
			id, name, progress, id,
		)
	}

	html += "</table>"

	html += `
	<script>
	async function nextEpisode(id) {
		await fetch("/update?id=" + id, { method: "POST" });
		location.reload();
	}
	</script>
	`

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
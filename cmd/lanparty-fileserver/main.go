package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	templatesGlobPath      = "templates/*.html"
	preloadedDir           = "./preloaded-games"
	uploadsDir             = "./uploads"
	downloadsLogDir        = "./downloads-log"
	port                   = "8080"
	defaultMaxUploadSizeMB = 100                  // Default max upload size in Megabytes
	envMaxUploadSizeMB     = "MAX_UPLOAD_SIZE_MB" // Environment variable name
)

// effectiveMaxUploadSizeBytes will store the actual max upload size to be used, in bytes.
var effectiveMaxUploadSizeBytes int64

type DownloadInfo struct {
	Timestamp    time.Time `json:"timestamp"`
	IPAddress    string    `json:"ipAddress"`
	UserAgent    string    `json:"userAgent"`
	FileName     string    `json:"fileName"`
	RequestedURL string    `json:"requestedUrl"`
}

type UploadResponse struct {
	Message string `json:"message"`
	Error   bool   `json:"error"`
}

type FileViewData struct {
	Name string
	Size int64
	URL  string
}

var templates *template.Template

func formatBytes(s int64) string {
	const unit = 1024
	if s < unit {
		return fmt.Sprintf("%d B", s)
	}
	div, exp := int64(unit), 0
	for n := s / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(s)/float64(div), "KMGTPE"[exp])
}

func main() {
	var err error
	funcMap := template.FuncMap{
		"formatBytes": formatBytes,
	}
	templates = template.Must(template.New("").Funcs(funcMap).ParseGlob(templatesGlobPath))

	maxUploadSizeMBStr := os.Getenv(envMaxUploadSizeMB)
	maxUploadSizeMB := defaultMaxUploadSizeMB
	if maxUploadSizeMBStr != "" {
		parsedMB, errConv := strconv.Atoi(maxUploadSizeMBStr)
		if errConv == nil && parsedMB > 0 {
			maxUploadSizeMB = parsedMB
			log.Printf("Using custom max upload size from %s: %d MB", envMaxUploadSizeMB, maxUploadSizeMB)
		} else {
			log.Printf("Warning: Invalid value for %s ('%s'). Using default: %d MB. Error: %v", envMaxUploadSizeMB, maxUploadSizeMBStr, defaultMaxUploadSizeMB, errConv)
		}
	} else {
		log.Printf("Using default max upload size: %d MB. Set %s to override.", defaultMaxUploadSizeMB, envMaxUploadSizeMB)
	}
	effectiveMaxUploadSizeBytes = int64(maxUploadSizeMB) << 20
	for _, dir := range []string{preloadedDir, uploadsDir, downloadsLogDir} {
		if err = os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Error creating directory %s: %v", dir, err)
		}
	}

	preloadGames()

	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/delete", deleteFileHandler)
	http.HandleFunc("/", rootHandler)

	log.Println("                                                 ")
	log.Println("██████╗ ██████╗ ██╗   ██╗███╗   ██╗██╗████████╗")
	log.Println("██╔══██╗╚════██╗██║   ██║████╗  ██║██║╚══██╔══╝")
	log.Println("██████╔╝ █████╔╝██║   ██║██╔██╗ ██║██║   ██║   ")
	log.Println("██╔══██╗██╔═══╝ ██║   ██║██║╚██╗██║██║   ██║   ")
	log.Println("██║  ██║███████╗╚██████╔╝██║ ╚████║██║   ██║   ")
	log.Println("╚═╝  ╚═╝╚══════╝ ╚═════╝ ╚═╝  ╚═══╝╚═╝   ╚═╝   ")
	log.Println("                                               ")
	log.Printf("Howdy! Your file server is on port %s...\n", port)
	log.Println("                                               ")
	log.Println("--- WARNING: File deletion is ENABLED without authentication! ---")
	log.Printf("Current date: %s", time.Now().Format(time.RFC1123))
	log.Printf("Maximum upload file size configured to: %d MB (%d bytes)", maxUploadSizeMB, effectiveMaxUploadSizeBytes)
	log.Printf("Access files by going to http://example:%s/", port)
	log.Printf("Access files by going to http://example:%s/upload", port)
	log.Println("--- Happy gaming away at the LAN party! 🎮 ---")
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func preloadGames() {
	log.Println("Scanning for preloaded games...")
	files, err := os.ReadDir(preloadedDir)
	if err != nil {
		log.Printf("Warning: Could not read preloaded games directory %s: %v", preloadedDir, err)
		return
	}
	for _, file := range files {
		if !file.IsDir() {
			srcPath := filepath.Join(preloadedDir, file.Name())
			dstPath := filepath.Join(uploadsDir, file.Name())
			if _, errStat := os.Stat(dstPath); errStat == nil {
				log.Printf("Skipping preload for %s: file already exists in uploads directory.", file.Name())
				continue
			}
			srcFile, errOpen := os.Open(srcPath)
			if errOpen != nil {
				log.Printf("Error opening preloaded file %s: %v", srcPath, errOpen)
				continue
			}
			deferFunc := func(f *os.File, path string) {
				if errClose := f.Close(); errClose != nil {
					log.Printf("Error closing file %s (in defer): %v", path, errClose)
				}
			}
			defer deferFunc(srcFile, srcPath)

			dstFile, errCreate := os.Create(dstPath)
			if errCreate != nil {
				log.Printf("Error creating destination file %s for preloading: %v", dstPath, errCreate)
				continue
			}
			defer deferFunc(dstFile, dstPath)

			_, errCopy := io.Copy(dstFile, srcFile)
			if errCopy != nil {
				log.Printf("Error copying preloaded file %s to %s: %v", srcPath, dstPath, errCopy)
			} else {
				log.Printf("Preloaded %s to %s", srcPath, dstPath)
			}
		}
	}
	log.Println("Preloading complete.")
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	requestedPath := filepath.Clean(r.URL.Path)
	if requestedPath == "/" || requestedPath == "." || requestedPath == "" {
		listFiles(w)
		return
	}
	fullPath := filepath.Join(uploadsDir, requestedPath)
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("File not found: %s (requested by %s for path: %s)", fullPath, r.RemoteAddr, r.URL.Path)
			http.NotFound(w, r)
		} else {
			log.Printf("Error accessing file %s (requested by %s): %v", fullPath, r.RemoteAddr, err)
			http.Error(w, "Internal server error checking file", http.StatusInternalServerError)
		}
		return
	}
	if fileInfo.IsDir() {
		log.Printf("Attempt to access directory listing for non-root by %s: %s", r.RemoteAddr, r.URL.Path)
		http.NotFound(w, r)
		return
	}
	logDownload(r, requestedPath)
	http.ServeFile(w, r, fullPath)
}

func listFiles(w http.ResponseWriter) {
	dirEntries, err := os.ReadDir(uploadsDir)
	if err != nil {
		log.Printf("Error reading uploads directory %s: %v", uploadsDir, err)
		http.Error(w, "Could not list files due to server error reading directory.", http.StatusInternalServerError)
		return
	}
	var filesView []FileViewData
	for _, entry := range dirEntries {
		if !entry.IsDir() {
			info, errInfo := entry.Info()
			if errInfo != nil {
				log.Printf("Error getting file info for %s: %v", entry.Name(), errInfo)
				continue
			}
			escapedName := url.PathEscape(info.Name())
			filesView = append(filesView, FileViewData{
				Name: info.Name(),
				Size: info.Size(),
				URL:  "/" + escapedName,
			})
		}
	}
	sort.Slice(filesView, func(i, j int) bool {
		return strings.ToLower(filesView[i].Name) < strings.ToLower(filesView[j].Name)
	})
	data := struct{ Files []FileViewData }{Files: filesView}
	err = templates.ExecuteTemplate(w, "list_files.html", data)
	if err != nil {
		log.Printf("Error executing list_files.html template: %v", err)
		http.Error(w, "Could not render file list due to template error.", http.StatusInternalServerError)
	}
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	isXHR := r.Header.Get("X-Requested-With") == "XMLHttpRequest"
	if r.Method == "GET" {
		renderUploadPage(w, "", false)
		return
	}
	if r.Method == "POST" {
		var responseMsg string
		var responseErr bool
		var httpStatusCode = http.StatusOK

		if err := r.ParseMultipartForm(effectiveMaxUploadSizeBytes); err != nil {
			log.Printf("Error parsing multipart form (or file too large, limit: %d bytes): %v", effectiveMaxUploadSizeBytes, err)
			responseMsg = fmt.Sprintf("Error parsing form (file might be too large, max %d MB): %s", effectiveMaxUploadSizeBytes>>20, err.Error())
			responseErr = true
			httpStatusCode = http.StatusBadRequest
		} else {
			file, handler, errFormFile := r.FormFile("fileToUpload")
			if errFormFile != nil {
				log.Printf("Error retrieving file from form: %v", errFormFile)
				responseMsg = "Error retrieving file: " + errFormFile.Error()
				responseErr = true
				httpStatusCode = http.StatusBadRequest
			} else {
				defer file.Close()
				fileName := filepath.Base(handler.Filename)
				if fileName == "." || fileName == "/" || fileName == "" {
					log.Printf("Invalid filename received: %s", handler.Filename)
					responseMsg = "Invalid filename provided."
					responseErr = true
					httpStatusCode = http.StatusBadRequest
				} else {
					filePath := filepath.Join(uploadsDir, fileName)
					if _, errStat := os.Stat(filePath); errStat == nil {
						log.Printf("File %s already exists. Upload aborted.", fileName)
						responseMsg = fmt.Sprintf("File '%s' already exists. Please rename and try again.", fileName)
						responseErr = true
						httpStatusCode = http.StatusConflict
					} else {
						dst, errCreate := os.Create(filePath)
						if errCreate != nil {
							log.Printf("Error creating file %s: %v", filePath, errCreate)
							responseMsg = "Error saving file (could not create): " + errCreate.Error()
							responseErr = true
							httpStatusCode = http.StatusInternalServerError
						} else {
							defer dst.Close()
							_, errCopy := io.Copy(dst, file)
							if errCopy != nil {
								log.Printf("Error copying uploaded file data for %s: %v", fileName, errCopy)
								responseMsg = "Error writing file data: " + errCopy.Error()
								responseErr = true
								httpStatusCode = http.StatusInternalServerError
								if errRemove := os.Remove(filePath); errRemove != nil {
									log.Printf("Additionally, failed to remove partially written file %s: %v", filePath, errRemove)
								}
							} else {
								log.Printf("Successfully uploaded %s to %s", fileName, filePath)
								responseMsg = fmt.Sprintf("File '%s' uploaded successfully!", fileName)
								responseErr = false
							}
						}
					}
				}
			}
		}
		if isXHR {
			sendJsonResponse(w, responseMsg, responseErr, httpStatusCode)
		} else {
			renderUploadPage(w, responseMsg, responseErr)
		}
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func deleteFileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJsonResponse(w, "Invalid request method. Only POST is allowed.", true, http.StatusMethodNotAllowed)
		return
	}
	if r.Header.Get("Content-Type") != "application/json" {
		sendJsonResponse(w, "Invalid Content-Type. Expected application/json.", true, http.StatusUnsupportedMediaType)
		return
	}
	var reqBody struct {
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		sendJsonResponse(w, "Invalid request body. Could not parse JSON.", true, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	if reqBody.Filename == "" {
		sendJsonResponse(w, "Filename cannot be empty.", true, http.StatusBadRequest)
		return
	}
	cleanFilename := filepath.Base(reqBody.Filename)
	if cleanFilename != reqBody.Filename || strings.ContainsAny(cleanFilename, "/\\") || strings.Contains(cleanFilename, "..") {
		log.Printf("Attempt to delete potentially malicious path: original '%s', cleaned '%s'", reqBody.Filename, cleanFilename)
		sendJsonResponse(w, "Invalid filename format or contains path characters.", true, http.StatusBadRequest)
		return
	}
	filePath := filepath.Join(uploadsDir, cleanFilename)
	absUploadsDir, errAbsUploads := filepath.Abs(uploadsDir)
	if errAbsUploads != nil {
		log.Printf("Critical error: Could not get absolute path for uploadsDir %s: %v", uploadsDir, errAbsUploads)
		sendJsonResponse(w, "Server configuration error preventing delete.", true, http.StatusInternalServerError)
		return
	}
	absFilePath, errAbsFile := filepath.Abs(filePath)
	if errAbsFile != nil {
		log.Printf("Critical error: Could not get absolute path for target file %s: %v", filePath, errAbsFile)
		sendJsonResponse(w, "Invalid filename leading to path error.", true, http.StatusInternalServerError)
		return
	}
	if !strings.HasPrefix(absFilePath, absUploadsDir+string(filepath.Separator)) || absFilePath == absUploadsDir {
		log.Printf("Security: Attempt to delete file/dir outside designated uploads area. Target: '%s', Uploads Dir: '%s'", absFilePath, absUploadsDir)
		sendJsonResponse(w, "Operation forbidden: file is outside designated area or is the uploads directory itself.", true, http.StatusForbidden)
		return
	}
	if _, errStat := os.Stat(filePath); os.IsNotExist(errStat) {
		log.Printf("File not found for deletion: %s", filePath)
		sendJsonResponse(w, fmt.Sprintf("File '%s' not found.", cleanFilename), true, http.StatusNotFound)
		return
	} else if errStat != nil {
		log.Printf("Error stating file before deletion %s: %v", filePath, errStat)
		sendJsonResponse(w, "Error checking file before deletion.", true, http.StatusInternalServerError)
		return
	}
	errRemove := os.Remove(filePath)
	if errRemove != nil {
		log.Printf("Error deleting file %s: %v", filePath, errRemove)
		sendJsonResponse(w, fmt.Sprintf("Failed to delete file '%s'. Check server logs.", cleanFilename), true, http.StatusInternalServerError)
		return
	}
	log.Printf("Successfully deleted file: %s by %s", filePath, r.RemoteAddr)
	sendJsonResponse(w, fmt.Sprintf("File '%s' deleted successfully.", cleanFilename), false, http.StatusOK)
}

func renderUploadPage(w http.ResponseWriter, message string, isError bool) {
	data := struct {
		Message string
		Error   bool
	}{Message: message, Error: isError}
	err := templates.ExecuteTemplate(w, "upload.html", data)
	if err != nil {
		log.Printf("Error executing upload.html template: %v", err)
		http.Error(w, "Error rendering upload page: "+err.Error(), http.StatusInternalServerError)
	}
}

func sendJsonResponse(w http.ResponseWriter, message string, hasError bool, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(UploadResponse{
		Message: message,
		Error:   hasError,
	})
}

func logDownload(r *http.Request, fileName string) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	forwardedFor := r.Header.Get("X-Forwarded-For")
	if forwardedFor != "" {
		ips := strings.Split(forwardedFor, ",")
		if len(ips) > 0 {
			clientIP := strings.TrimSpace(ips[0])
			parsedIP := net.ParseIP(clientIP)
			if parsedIP != nil {
				ip = clientIP
			} else {
				log.Printf("Warning: could not parse IP from X-Forwarded-For: %s", clientIP)
			}
		}
	}
	if host, _, errSplit := net.SplitHostPort(ip); errSplit == nil {
		ip = host
	}
	info := DownloadInfo{
		Timestamp:    time.Now(),
		IPAddress:    ip,
		UserAgent:    r.UserAgent(),
		FileName:     filepath.Base(fileName),
		RequestedURL: r.URL.String(),
	}
	jsonData, errJson := json.MarshalIndent(info, "", "  ")
	if errJson != nil {
		log.Printf("Error marshalling download info for %s: %v", info.FileName, errJson)
		return
	}
	safeFileNameForLog := strings.ReplaceAll(info.FileName, ".", "_")
	safeFileNameForLog = strings.ReplaceAll(safeFileNameForLog, string(filepath.Separator), "_")
	logFileName := fmt.Sprintf("%s_%s.json", time.Now().Format("20060102150405"), safeFileNameForLog)
	logFilePath := filepath.Join(downloadsLogDir, logFileName)
	if errWrite := os.WriteFile(logFilePath, jsonData, 0644); errWrite != nil {
		log.Printf("Error writing download log for %s to %s: %v", info.FileName, logFilePath, errWrite)
	} else {
		log.Printf("Logged download of %s by %s to %s", info.FileName, info.IPAddress, logFilePath)
	}
}

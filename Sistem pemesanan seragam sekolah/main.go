package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jung-kurt/gofpdf"
)

type Pesanan struct {
	Id           int
	Nama         string
	Kelas        string
	Ukuran       string
	Jenis        string
	Jumlah       int
	JenisKelamin string
}

type Summary struct {
	Pesanan []Pesanan

	TotalBatikL int
	TotalBatikP int

	TotalRompiL int
	TotalRompiP int

	TotalKabupatenL int
	TotalKabupatenP int

	TotalOlahragaL int
	TotalOlahragaP int

	TotalKeseluruhan int
}

var db *sql.DB

func main() {
	var err error
	db, err = sql.Open("mysql", "root:@tcp(127.0.0.1:3306)/pemesanan_seragam_tk")
	if err != nil {
		log.Fatal("Gagal koneksi database:", err)
	}
	defer db.Close()

	http.HandleFunc("/", formHandler)
	http.HandleFunc("/cek-pesanan", cekPesananHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/proses-login", prosesLoginHandler)
	http.HandleFunc("/admin", adminHandler)
	http.HandleFunc("/edit", editHandler)
	http.HandleFunc("/delete", deleteHandler)
	http.HandleFunc("/cetak-pdf", cetakPDFHandler)
	http.HandleFunc("/pesanan-terkirim", pesananTerkirimHandler)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	fmt.Println("Server berjalan di http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// FORM INPUT
func formHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Gagal parsing form", http.StatusBadRequest)
			return
		}

		jumlah, err := strconv.Atoi(r.FormValue("jumlah"))
		if err != nil {
			http.Error(w, "Jumlah tidak valid", http.StatusBadRequest)
			return
		}

		_, err = db.Exec("INSERT INTO pesanan (nama, kelas, ukuran, jenis, jumlah, jenis_kelamin) VALUES (?, ?, ?, ?, ?, ?)",
			r.FormValue("nama"), r.FormValue("kelas"), r.FormValue("ukuran"), r.FormValue("jenis"), jumlah, r.FormValue("jenis_kelamin"))
		if err != nil {
			http.Error(w, "Gagal simpan data", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/pesanan-terkirim", http.StatusSeeOther)
		return
	}

	tmpl := template.Must(template.ParseFiles("templates/form.html"))
	tmpl.Execute(w, nil)
}

// LOGIN
func loginHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/login.html"))
	tmpl.Execute(w, nil)
}

// LOGIN PROSES
func prosesLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	var dbUsername, dbPassword string
	err := db.QueryRow("SELECT username, password FROM admin WHERE username = ?", username).Scan(&dbUsername, &dbPassword)
	if err != nil || password != dbPassword {
		tmpl := template.Must(template.ParseFiles("templates/login.html"))
		tmpl.Execute(w, "Username atau password salah")
		return
	}

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func pesananTerkirimHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/pesanan_terkirim.html"))
	tmpl.Execute(w, nil)
}

// ADMIN
func adminHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT * FROM pesanan")
	if err != nil {
		http.Error(w, "Gagal mengambil data", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var pesananList []Pesanan
	var batikL, batikP, rompiL, rompiP, kabupatenL, kabupatenP, olahragaL, olahragaP int

	for rows.Next() {
		var p Pesanan
		err := rows.Scan(&p.Id, &p.Nama, &p.Kelas, &p.Ukuran, &p.Jenis, &p.Jumlah, &p.JenisKelamin)
		if err != nil {
			log.Println("Gagal membaca baris:", err)
			continue
		}
		pesananList = append(pesananList, p)

		switch p.Jenis {
		case "Batik":
			if p.JenisKelamin == "L" {
				batikL += p.Jumlah
			} else {
				batikP += p.Jumlah
			}
		case "Harian Rompi":
			if p.JenisKelamin == "L" {
				rompiL += p.Jumlah
			} else {
				rompiP += p.Jumlah
			}
		case "Harian Kabupaten":
			if p.JenisKelamin == "L" {
				kabupatenL += p.Jumlah
			} else {
				kabupatenP += p.Jumlah
			}
		case "Olahraga":
			if p.JenisKelamin == "L" {
				olahragaL += p.Jumlah
			} else {
				olahragaP += p.Jumlah
			}
		}
	}

	total := batikL + batikP + rompiL + rompiP + kabupatenL + kabupatenP + olahragaL + olahragaP

	data := Summary{
		Pesanan:          pesananList,
		TotalBatikL:      batikL,
		TotalBatikP:      batikP,
		TotalRompiL:      rompiL,
		TotalRompiP:      rompiP,
		TotalKabupatenL:  kabupatenL,
		TotalKabupatenP:  kabupatenP,
		TotalOlahragaL:   olahragaL,
		TotalOlahragaP:   olahragaP,
		TotalKeseluruhan: total,
	}

	tmpl := template.Must(template.ParseFiles("templates/admin.html"))
	tmpl.Execute(w, data)
}

// EDIT
func editHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodPost {
		r.ParseForm()
		jumlah, _ := strconv.Atoi(r.FormValue("jumlah"))
		_, err := db.Exec("UPDATE pesanan SET nama=?, kelas=?, ukuran=?, jenis=?, jumlah=?, jenis_kelamin=? WHERE id=?",
			r.FormValue("nama"), r.FormValue("kelas"), r.FormValue("ukuran"), r.FormValue("jenis"), jumlah, r.FormValue("jenis_kelamin"), id)
		if err != nil {
			http.Error(w, "Gagal update data", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	var p Pesanan
	err = db.QueryRow("SELECT * FROM pesanan WHERE id = ?", id).Scan(&p.Id, &p.Nama, &p.Kelas, &p.Ukuran, &p.Jenis, &p.Jumlah, &p.JenisKelamin)
	if err != nil {
		http.Error(w, "Data tidak ditemukan", http.StatusNotFound)
		return
	}

	tmpl := template.Must(template.ParseFiles("templates/edit.html"))
	tmpl.Execute(w, p)
}

// DELETE
func deleteHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, _ := strconv.Atoi(idStr)

	_, err := db.Exec("DELETE FROM pesanan WHERE id=?", id)
	if err != nil {
		http.Error(w, "Gagal hapus data", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// CEK PESANAN
func cekPesananHandler(w http.ResponseWriter, r *http.Request) {
	nama := r.URL.Query().Get("nama")
	var pesananList []Pesanan

	if nama != "" {
		rows, err := db.Query("SELECT nama, kelas, ukuran, jenis, jumlah, jenis_kelamin FROM pesanan WHERE nama = ?", nama)
		if err != nil {
			http.Error(w, "Gagal mengambil data", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var p Pesanan
			err := rows.Scan(&p.Nama, &p.Kelas, &p.Ukuran, &p.Jenis, &p.Jumlah, &p.JenisKelamin)
			if err != nil {
				http.Error(w, "Gagal memproses data", http.StatusInternalServerError)
				return
			}
			pesananList = append(pesananList, p)
		}
	}

	data := struct {
		Nama        string
		PesananList []Pesanan
	}{
		Nama:        nama,
		PesananList: pesananList,
	}

	tmpl := template.Must(template.ParseFiles("templates/cek_pesanan.html"))
	tmpl.Execute(w, data)
}

// CETAK PDF
func cetakPDFHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT * FROM pesanan")
	if err != nil {
		http.Error(w, "Gagal mengambil data", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(10, 10, 10) // kiri, atas, kanan
	pdf.AddPage()

	// Judul
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(0, 10, "Daftar Pesanan Seragam TK", "", 1, "C", false, 0, "")
	pdf.Ln(5)

	// Header tabel dan lebar kolom (total 257 mm)
	headers := []string{"Nama", "Kelas", "Ukuran", "Jenis", "Jumlah", "Jenis Kelamin"}
	colWidths := []float64{50, 30, 25, 75, 27, 50} // Total 257 mm (pas)

	pdf.SetFont("Arial", "B", 11)
	for i, h := range headers {
		pdf.CellFormat(colWidths[i], 7, h, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	// Font isi tabel
	pdf.SetFont("Arial", "", 10)

	// Ringkasan data
	var batikL, batikP, rompiL, rompiP, kabupatenL, kabupatenP, olahragaL, olahragaP int

	for rows.Next() {
		var p Pesanan
		err := rows.Scan(&p.Id, &p.Nama, &p.Kelas, &p.Ukuran, &p.Jenis, &p.Jumlah, &p.JenisKelamin)
		if err != nil {
			continue
		}

		// Isi baris tabel
		values := []string{p.Nama, p.Kelas, p.Ukuran, p.Jenis, strconv.Itoa(p.Jumlah), p.JenisKelamin}
		for i, val := range values {
			pdf.CellFormat(colWidths[i], 7, val, "1", 0, "", false, 0, "")
		}
		pdf.Ln(-1)

		// Hitung total jenis kelamin per jenis seragam
		switch p.Jenis {
		case "Batik":
			if p.JenisKelamin == "L" {
				batikL += p.Jumlah
			} else {
				batikP += p.Jumlah
			}
		case "Harian Rompi":
			if p.JenisKelamin == "L" {
				rompiL += p.Jumlah
			} else {
				rompiP += p.Jumlah
			}
		case "Harian Kabupaten":
			if p.JenisKelamin == "L" {
				kabupatenL += p.Jumlah
			} else {
				kabupatenP += p.Jumlah
			}
		case "Olahraga":
			if p.JenisKelamin == "L" {
				olahragaL += p.Jumlah
			} else {
				olahragaP += p.Jumlah
			}
		}
	}

	total := batikL + batikP + rompiL + rompiP + kabupatenL + kabupatenP + olahragaL + olahragaP

	// Ringkasan Total
	pdf.Ln(8)
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(0, 7, "Ringkasan Total:", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 6, fmt.Sprintf("Total Batik - L: %d, P: %d", batikL, batikP), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Total Harian Rompi - L: %d, P: %d", rompiL, rompiP), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Total Harian Kabupaten - L: %d, P: %d", kabupatenL, kabupatenP), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Total Olahraga - L: %d, P: %d", olahragaL, olahragaP), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Total Keseluruhan: %d", total), "", 1, "L", false, 0, "")

	// Output ke browser
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "inline; filename=pesanan.pdf")
	err = pdf.Output(w)
	if err != nil {
		http.Error(w, "Gagal membuat PDF", http.StatusInternalServerError)
	}
}

package dimulai

import (
	"os"
	"testing"

	"github.com/dimframework/dim"
)

// TestMain menurunkan cost bcrypt untuk seluruh test paket ini.
//
// Bawaan dim adalah 12 — tepat untuk produksi, tetapi mahal di test. bcrypt Go
// murni adalah gelung akses memori yang rapat, dan race detector
// menginstrumentasi setiap akses: satu hash memakan ±200 ms, dan ±2 dtk di
// bawah `-race`. Test di sini mendaftar, login, dan mengganti kata sandi
// berkali-kali, sehingga biayanya menumpuk sampai mendekati batas 10 menit
// bawaan `go test`.
//
// Cost 6 tetap bcrypt sungguhan — jalur hashing yang diuji sama persis, hanya
// putarannya lebih sedikit. Jangan menurunkannya di produksi, dan jangan
// menyambungkannya ke config atau environment.
func TestMain(m *testing.M) {
	dim.BcryptCost = 6
	os.Exit(m.Run())
}

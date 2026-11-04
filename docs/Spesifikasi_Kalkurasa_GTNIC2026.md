# Spesifikasi Kebutuhan Produk: Kalkurasa

Versi 0.1, draf untuk didiskusikan · 4 November 2026 · GTNIC 2026 cabang Web Development

Dokumen ini hanya memuat kebutuhan produk. Arsitektur, skema database, dan pembagian tugas dibahas terpisah setelah spesifikasi disepakati. Tanda (H) berarti hipotesis atau asumsi yang belum divalidasi. Semua angka contoh adalah SIMULASI untuk usaha fiktif “Dapur Bu Sari”.

## 1. Ringkasan produk

- **Nama produk:** Kalkurasa (dari “kalkulasi” dan “rasa”: usaha makanan yang dihitung dengan tepat tanpa kehilangan rasa).
- **Vertikal:** UMKM kuliner berbasis pesanan di muka (kue pre-order, snack box, katering box) dengan sekitar 5–20 orang (H).
- **Masalah (H):** pemilik menerima pesanan tanpa tahu apakah kapasitas produksi, margin, dan kas sanggup menanggungnya, sehingga muncul pesanan telat, margin menipis, belanja bahan salah hitung, dan kas tertekan.
- **Solusi:** pesanan hanya bisa dibuat pada tanggal yang masih muat. Sistem menghitung biaya dan margin, merekomendasikan terima, terima bersyarat, atau tolak beserta alasan dan tawaran balik, lalu menggerakkan produksi, belanja, kas, dan pengantaran dalam satu alur dengan dashboard per peran.
- **Prinsip:** sistem merekomendasikan, manusia memutuskan. Semua rumus dapat dijelaskan. Tanpa AI. Satu alur utama harus berjalan penuh sebelum fitur lain dikerjakan.
- **Pembeda (H, belum terbukti):** kapasitas dihitung dari beban sumber daya lintas produk, lalu tersambung ke keputusan, daftar belanja, dan proyeksi kas. Platform pemesanan lain sudah membatasi pesanan per slot, jadi kita tidak mengklaim “baru”.

## 2. Dasar dari brief lomba

| Item | Isi resmi dari brief |
|---|---|
| Tema | Accelerating Digital Transformation for Business and SMEs (Business, UMKM, Digital Economy) |
| Jadwal | Deadline 10 Nov 2026 pukul 23.59 WIB; pengumuman Top 5 pada 12 Nov; Grand Final dan Exhibition via Zoom pada 14 Nov |
| Pengumpulan | Website live publik; repository GitHub publik; proposal PDF maks. 15 halaman (A4, Times New Roman 12 pt, spasi 1,5, margin kiri 4 cm dan lainnya 3 cm, logo GTNIC 2026 di cover); video demo 3–5 menit publik; Surat Pernyataan Keaslian Karya; nama file GTNIC2026_WebDev_NamaTim.pdf |
| Teknis | Bukan mockup; data tersimpan di database; form, validasi, tambah/ubah/hapus; alur utama end-to-end; responsif desktop, tablet, ponsel; uji browser utama; feedback, loading, dan pesan error; keamanan dasar; tanpa kredensial di repo; README lengkap; data seed realistis; akun demo untuk juri tanpa rahasia sensitif |
| Orisinalitas | AI untuk membantu coding boleh jika tim memahami dan bisa mempertanggungjawabkan hasilnya; data simulasi harus ditandai jelas |

| Kriteria rubrik (bobot) | Cara spesifikasi ini menjawabnya |
|---|---|
| Fungsionalitas dan implementasi (25%) | Satu alur end-to-end nyata (Bagian 5); semua modul Must terhubung ke data |
| Relevansi masalah dan inovasi (20%) | Masalah kapasitas, margin, dan kas pada usaha pre-order; keputusan disertai alasan |
| Teknis dan arsitektur (20%) | Integritas data lintas modul, anti-overbooking, rumus teruji (NFR dan Bagian 13) |
| UX/UI (15%) | Dashboard per peran; status loading, kosong, error; responsif |
| Dampak bisnis dan skalabilitas (10%) | KPI dan simulasi 30 hari berlabel (Bagian 10) |
| Dokumentasi, demo, presentasi (10%) | README, seed, akun demo, skenario demo (Bagian 12 dan 14) |

## 3. Ruang lingkup (MoSCoW)

| Prioritas | Isi |
|---|---|
| **Must** | Login dan peran; master produk, bahan, resep, beban produksi, sumber daya, parameter; form pesanan publik dengan pemilih tanggal; mesin kapasitas; biaya dan margin; mesin keputusan dengan pratinjau dampak; pembayaran manual (DP, pelunasan); jadwal dan tugas produksi; daftar belanja dan stok; proyeksi kas; dashboard per peran; data seed dan akun demo |
| **Should** | Kalibrasi beban produksi; simulasi 30 hari dan pengendali waktu demo; kalender libur; notifikasi dalam aplikasi; audit log lengkap; penjadwalan ulang produksi; jadwal antar; pratinjau “bagaimana jika” |
| **Could** | Unggah bukti bayar; permintaan pembatalan oleh pelanggan; ekspor laporan; log kesalahan server |
| **Won't (versi ini)** | Payment gateway; integrasi WhatsApp atau API pihak ketiga; payroll dan HR; akuntansi lengkap dan pajak; multi-cabang; aplikasi mobile native; AI atau LLM |

## 4. Peran dan hak akses

- **Pelanggan** (publik, tanpa login): membuat pesanan dan melacaknya dengan kode.
- **Karyawan:** mengerjakan tugas produksi, belanja, dan antar.
- **Manajer produksi:** mengelola jadwal, master data, dan memutuskan pesanan di bawah ambang pesanan besar.
- **Pemilik:** memutuskan pesanan besar, mengatur parameter bisnis, melihat kas dan KPI.
- **Admin:** mengelola pengguna, data dasar, dan data demo. Admin tidak memutuskan pesanan.

Keterangan: ✓ penuh, R hanya lihat, – tidak ada akses.

| Fitur | Pelanggan | Karyawan | Manajer | Pemilik | Admin |
|---|---|---|---|---|---|
| Form pesanan dan cek tanggal | ✓ | – | ✓ (pesanan manual) | ✓ | – |
| Lacak pesanan dengan kode | ✓ | R | ✓ | ✓ | R |
| Tugas produksi, belanja, antar | – | ✓ | ✓ (kelola) | R | R |
| Keputusan pesanan di bawah ambang | – | – | ✓ | ✓ | – |
| Keputusan pesanan besar | – | – | usul | ✓ | – |
| Override rekomendasi (alasan wajib) | – | – | ✓ (di bawah ambang) | ✓ | – |
| Catat pembayaran | – | – | ✓ | ✓ | – |
| Produk, resep, bahan, beban produksi | – | – | ✓ | R | ✓ |
| Parameter bisnis | – | – | R | ✓ | – |
| Kas dan proyeksi | – | – | status lolos atau tidak saja | ✓ | – |
| KPI dan kalibrasi | – | – | ✓ | ✓ | R |
| Pengguna dan peran | – | – | – | – | ✓ |
| Audit log | – | – | – | R | ✓ |
| Reset data demo dan mode waktu | – | – | – | – | ✓ |

Karyawan tidak melihat harga jual, margin, dan kas.

## 5. Alur penggunaan

**Alur utama AU-1 (harus berjalan penuh dari awal sampai akhir):**

1. Pelanggan memilih produk, jumlah, dan tanggal. Pemilih tanggal menandai hari penuh atau tidak tersedia.
2. Pelanggan mengirim pesanan dan mendapat kode pesanan serta halaman lacak.
3. Sistem mengevaluasi gerbang G1–G4 (Bagian 8).
4. Jika semua gerbang lolos dan nilai di bawah ambang pesanan besar, pesanan langsung Diterima. Jika tidak, pesanan masuk antrian keputusan Manajer atau Pemilik dengan pratinjau dampak.
5. Pemilik atau Manajer menerima, menawar balik, atau menolak. Kapasitas direservasi sejak Diterima.
6. Sistem menagih DP. Staf mencatat pembayaran manual dan pesanan menjadi Terkonfirmasi.
7. Sistem membuat tugas produksi, belanja, dan antar, serta memperbarui daftar belanja dan proyeksi kas.
8. Karyawan mengerjakan tugas dan memperbarui status. Stok berkurang saat produksi selesai.
9. Pesanan Siap lalu diantar atau diambil. Pelunasan dicatat dan pesanan Selesai.
10. KPI dan kalibrasi diperbarui.

**Alur sekunder:** AS-1 pesanan manual dari chat oleh Manajer; AS-2 pembatalan (kapasitas dilepas, bahan menjadi stok, kas dihitung ulang); AS-3 pelanggan menerima atau menolak tawaran balik; AS-4 saran kalibrasi disetujui Manajer.

## 6. Status pesanan

| Status | Arti | Menuju | Pelaku |
|---|---|---|---|
| Diajukan | Pesanan baru masuk | Menunggu Keputusan atau Diterima | Sistem |
| Menunggu Keputusan | Antrian keputusan | Diterima, Ditawar Balik, Ditolak | Manajer atau Pemilik |
| Ditawar Balik | Tawaran tanggal, harga, atau DP dikirim | Diterima atau Ditolak | Pelanggan |
| Diterima (Menunggu DP) | Kapasitas direservasi, DP ditagih | Terkonfirmasi, Kedaluwarsa, Dibatalkan | Staf, sistem |
| Terkonfirmasi | DP tercatat, tugas dibuat | Dalam Produksi, Dibatalkan | Staf |
| Dalam Produksi | Produksi berjalan | Siap | Karyawan |
| Siap | Selesai diproduksi | Diantar atau Siap Diambil | Karyawan |
| Diantar atau Siap Diambil | Dalam penyerahan | Selesai | Karyawan |
| Selesai | Lunas dan diserahkan | Final | Staf |
| Ditolak, Dibatalkan, Kedaluwarsa | Kapasitas dan reservasi dilepas | Final | Staf, sistem |

## 7. Kebutuhan fungsional

Prioritas: M Must, S Should, C Could, W Won't.

**Autentikasi dan pengguna**

| ID | Kebutuhan | P |
|---|---|---|
| FR-AUTH-01 | Login dengan email dan kata sandi untuk empat peran internal; logout; sesi aman | M |
| FR-AUTH-02 | Hak akses ditegakkan di server; aksi tanpa izin ditolak dengan pesan jelas | M |
| FR-AUTH-03 | Admin membuat, menonaktifkan, dan mengubah peran pengguna | M |
| FR-AUTH-04 | Akun demo tiap peran (data uji) dengan instruksi di README tanpa kredensial sensitif | M |

**Master data**

| ID | Kebutuhan | P |
|---|---|---|
| FR-MD-01 | CRUD produk: nama, kategori, harga jual, satuan, status aktif | M |
| FR-MD-02 | CRUD bahan: satuan pakai, satuan beli dan isinya, harga per satuan beli, stok, stok minimum | M |
| FR-MD-03 | Resep per satu unit produk: bahan, takaran, kemasan | M |
| FR-MD-04 | Beban produksi tiap produk per sumber daya (oven-jam, menit tenaga) | M |
| FR-MD-05 | CRUD sumber daya: nama, jumlah, jam per hari, efisiensi (awal: oven dan tenaga kerja) | M |
| FR-MD-06 | Parameter bisnis sesuai tabel 8.2, dapat diubah Pemilik | M |
| FR-MD-07 | Kalender libur dan penyesuaian kapasitas harian | S |
| FR-MD-08 | Validasi: nilai tidak negatif, satuan wajib; produk nonaktif tidak bisa dipesan; data master yang terpakai diarsipkan, bukan dihapus | M |

**Pemesanan**

| ID | Kebutuhan | P |
|---|---|---|
| FR-ORD-01 | Katalog dan form pesanan publik tanpa login: produk, jumlah, tanggal, nama, kontak, antar atau ambil, alamat, catatan | M |
| FR-ORD-02 | Pemilih tanggal menampilkan status per hari (tersedia, hampir penuh, penuh, di bawah lead time) berdasarkan isi pesanan saat itu | M |
| FR-ORD-03 | Estimasi total harga sebelum kirim | M |
| FR-ORD-04 | Validasi klien dan server (wajib isi, jumlah bulat lebih dari 0, format kontak, tanggal sesuai lead time) dengan pesan spesifik | M |
| FR-ORD-05 | Kode pesanan unik yang tidak mudah ditebak dan halaman lacak: status, tagihan DP, tenggat, instruksi bayar manual | M |
| FR-ORD-06 | Pembatasan laju pada form publik | S |
| FR-ORD-07 | Pelanggan menerima atau menolak tawaran balik di halaman lacak | M |
| FR-ORD-08 | Manajer atau Pemilik membuat pesanan manual atas nama pelanggan | M |
| FR-ORD-09 | Permintaan pembatalan oleh pelanggan | C |

**Kapasitas**

| ID | Kebutuhan | P |
|---|---|---|
| FR-CAP-01 | Hitung kapasitas, beban, dan utilisasi per sumber daya per hari (BR-01 sampai BR-04) | M |
| FR-CAP-02 | Kalender kapasitas bulanan berwarna dengan rincian dan sumber daya penghambat | M |
| FR-CAP-03 | Kapasitas direservasi sejak Diterima, dilepas saat Ditolak, Dibatalkan, atau Kedaluwarsa | M |
| FR-CAP-04 | Anti-overbooking: dua pesanan bersamaan tidak boleh sama-sama melewati batas terima pada slot yang tersisa | M |
| FR-CAP-05 | Beban ditempatkan pada tanggal produksi (default sama dengan tanggal kirim); Manajer dapat menggeser | S |

**Biaya dan margin**

| ID | Kebutuhan | P |
|---|---|---|
| FR-COST-01 | Hitung biaya per unit dan margin per produk dan per pesanan (BR-05, BR-06) | M |
| FR-COST-02 | Tampilkan harga minimal dan diskon maksimum | M |
| FR-COST-03 | Biaya, harga, dan parameter dibekukan (snapshot) pada pesanan saat keputusan; perubahan harga bahan tidak mengubah pesanan lama | M |

**Keputusan**

| ID | Kebutuhan | P |
|---|---|---|
| FR-DEC-01 | Setiap pesanan memicu evaluasi G1–G4; hasil (nilai, ambang, lolos atau tidak) disimpan | M |
| FR-DEC-02 | Rekomendasi Terima, Terima bersyarat, atau Tolak dengan alasan yang terbaca manusia | M |
| FR-DEC-03 | Tawaran balik otomatis: tanggal terdekat yang muat, harga minimal, DP minimal | M |
| FR-DEC-04 | Aturan wewenang: semua gerbang lolos dan di bawah ambang pesanan besar berarti Diterima otomatis; selain itu masuk antrian sesuai peran | M |
| FR-DEC-05 | Override rekomendasi mewajibkan alasan dan tercatat di audit log (siapa, kapan, rekomendasi awal) | M |
| FR-DEC-06 | Antrian keputusan memuat pratinjau dampak: utilisasi, margin, dan titik terendah kas sebelum dan sesudah | M |
| FR-DEC-07 | Pratinjau “bagaimana jika” saat mengubah tanggal, harga, atau DP | S |

**Pembayaran**

| ID | Kebutuhan | P |
|---|---|---|
| FR-PAY-01 | Tagihan DP otomatis: nilai terbesar antara persen DP default dan DP minimal dari model kas; tenggat sesuai parameter | M |
| FR-PAY-02 | Staf mencatat pembayaran manual (jumlah, tanggal, metode, catatan); status belum, DP, atau lunas | M |
| FR-PAY-03 | Pesanan menjadi Terkonfirmasi hanya setelah DP tercatat | M |
| FR-PAY-04 | Pelunasan, sisa tagihan, dan penyelesaian pesanan | M |
| FR-PAY-05 | Pembatalan: hitung eksposur; keputusan DP hangus atau dikembalikan dicatat oleh manusia | S |
| FR-PAY-06 | Unggah bukti bayar | C |
| FR-PAY-07 | Payment gateway | W |

**Produksi dan tugas**

| ID | Kebutuhan | P |
|---|---|---|
| FR-PRD-01 | Jadwal produksi harian dari pesanan terkonfirmasi | M |
| FR-PRD-02 | Tugas otomatis (produksi, belanja, antar atau ambil) dengan tenggat, penerima, dan status | M |
| FR-PRD-03 | Karyawan melihat dan memperbarui status tugas; status pesanan mengikuti | M |
| FR-PRD-04 | Karyawan mencatat durasi aktual per pesanan | S |
| FR-PRD-05 | Manajer menugaskan ulang tugas | S |
| FR-PRD-06 | Daftar antar harian terurut menurut waktu | S |

**Bahan, belanja, stok**

| ID | Kebutuhan | P |
|---|---|---|
| FR-STK-01 | Hitung kebutuhan bahan per hari dan per rentang dari pesanan terkonfirmasi (BR-07) | M |
| FR-STK-02 | Daftar belanja: kebutuhan, stok, buffer, jumlah beli per satuan beli, estimasi biaya, tanggal beli | M |
| FR-STK-03 | Tandai sudah dibeli dengan jumlah dan harga aktual; stok dan kas diperbarui | M |
| FR-STK-04 | Stok berkurang saat produksi selesai; koreksi stok manual dengan alasan | M |
| FR-STK-05 | Peringatan stok di bawah minimum | S |
| FR-STK-06 | Pesanan batal: bahan yang sudah dibeli menjadi stok | M |

**Kas**

| ID | Kebutuhan | P |
|---|---|---|
| FR-CASH-01 | Proyeksi saldo harian 14 hari dari saldo awal, pemasukan terjadwal, belanja terjadwal, dan biaya tetap (BR-08) | M |
| FR-CASH-02 | Tampilkan titik terendah, tanggalnya, dan hari di bawah buffer | M |
| FR-CASH-03 | Catat kas manual (pemasukan dan pengeluaran lain) | S |
| FR-CASH-04 | Eksposur pembatalan per pesanan (BR-11) | S |

**KPI, kalibrasi, simulasi, audit, notifikasi**

| ID | Kebutuhan | P |
|---|---|---|
| FR-KPI-01 | KPI dihitung dari data transaksi (Bagian 10) | M |
| FR-KAL-01 | Deviasi beban aktual terhadap estimasi; saran koreksi saat memenuhi BR-12; penerapan butuh persetujuan Manajer | S |
| FR-SIM-01 | Simulasi 30 hari dengan aliran pesanan sintetis (seed tetap), dijalankan dengan dan tanpa sistem, berlabel SIMULASI | S |
| FR-SIM-02 | Pengendali tanggal sistem untuk demo, hanya Admin | S |
| FR-AUD-01 | Audit log untuk keputusan, override, perubahan parameter, pembayaran, dan perubahan status | M |
| FR-NTF-01 | Notifikasi dalam aplikasi: tugas baru, keputusan menunggu, DP jatuh tempo | S |

## 8. Aturan bisnis dan parameter

**8.1 Aturan dan rumus**

| ID | Aturan |
|---|---|
| BR-01 | Kapasitas sumber daya r = jumlah × jam per hari × efisiensi (oven dalam oven-jam, tenaga dalam menit) |
| BR-02 | Beban r pada satu hari = jumlah dari (qty produk × beban produk pada r) |
| BR-03 | Utilisasi r = beban ÷ kapasitas; utilisasi hari = nilai tertinggi antar sumber daya; sumber daya tertinggi adalah penghambat |
| BR-04 | Status hari: hijau di bawah 70%, kuning 70% sampai di bawah 90%, merah 90% sampai 100%, penuh di atas 100% (ambang dapat diatur) |
| BR-05 | Biaya per unit = bahan × (1 + waste) + kemasan + menit tenaga × tarif per menit + overhead |
| BR-06 | Margin = (harga − biaya) ÷ harga; harga minimal = biaya ÷ (1 − margin minimum); diskon maksimum = 1 − harga minimal ÷ harga normal |
| BR-07 | Kebutuhan bahan = jumlah (qty × takaran); perlu beli = maks(0, kebutuhan × (1 + buffer) − stok), dibulatkan ke atas ke satuan beli sesuai aturan bahan (eceran atau kemasan penuh) |
| BR-08 | Saldo hari t = saldo t−1 + pemasukan − belanja − biaya tetap; titik terendah = nilai minimum pada periode |
| BR-09 | DP minimal = maks(0, buffer kas − titik terendah tanpa DP); DP ditagih = maks(persen DP default × total, DP minimal); jika DP minimal melebihi total pesanan, G3 gagal tanpa perbaikan |
| BR-10 | G1: utilisasi setelah menerima ≤ batas terima. G2: margin ≥ margin minimum. G3: titik terendah kas dengan DP ditagih ≥ buffer kas. G4: jarak hari ke tanggal ≥ lead time. Semua lolos berarti Terima. G4 lolos dan G1, G2, atau G3 gagal tetapi ada perbaikan berarti Terima bersyarat. G4 gagal atau tidak ada perbaikan berarti Tolak |
| BR-11 | Eksposur pembatalan = maks(0, belanja terpesan − DP diterima) |
| BR-12 | Deviasi = (rata-rata aktual − estimasi) ÷ estimasi; saran koreksi jika nilai mutlak di atas 10% dan sampel minimal 5 |
| BR-13 | Snapshot harga, biaya, parameter, dan hasil gerbang disimpan pada pesanan |
| BR-14 | Kapasitas direservasi sejak Diterima; pesanan Kedaluwarsa jika DP tidak tercatat sebelum tenggat |
| BR-15 | Uang dibulatkan ke rupiah penuh; persentase ditampilkan satu desimal; hitungan internal tanpa pembulatan |

**8.2 Parameter awal (semua asumsi yang harus divalidasi)**

| Parameter | Nilai awal | Pengatur |
|---|---|---|
| Oven | 2 unit × 8 jam | Admin dan Manajer |
| Tenaga kerja | 4 orang × 420 menit | Admin dan Manajer |
| Efisiensi | 85% | Manajer |
| Batas terima kapasitas | 90% | Pemilik |
| Margin minimum | 20% | Pemilik |
| Tarif tenaga per menit | Rp 400 | Pemilik |
| Waste bahan, buffer belanja | 5%, 10% | Manajer |
| Buffer kas | 1 hari biaya tetap | Pemilik |
| Lead time minimum | 2 hari | Pemilik |
| DP default, tenggat DP | 30%, 24 jam | Pemilik |
| Ambang pesanan besar | total ≥ Rp 2.000.000 atau ≥ 50 unit | Pemilik |

## 9. Dashboard per peran

| Peran | Isi utama (maksimal lima elemen) |
|---|---|
| Pemilik | Keputusan menunggu dengan pratinjau dampak; proyeksi kas 14 hari dengan hari rawan; margin dan KPI ringkas; kalender kapasitas ringkas |
| Manajer | Kalender kapasitas; antrian keputusan di bawah ambang; jadwal produksi dan daftar belanja; peringatan stok dan tenggat DP |
| Karyawan | Tugas hari ini; pembaruan status; input durasi aktual; daftar antar |
| Admin | Pengguna dan peran; data dasar; reset data demo dan pengendali tanggal; audit log |
| Pelanggan | Pemilih tanggal dan halaman lacak pesanan |

## 10. KPI dan pengukuran dampak

| KPI | Definisi |
|---|---|
| Ketepatan waktu | Pesanan selesai paling lambat tanggal acara ÷ pesanan selesai |
| Utilisasi rata-rata | Rata-rata utilisasi hari produksi |
| Selisih biaya bahan | (biaya aktual − estimasi) ÷ estimasi |
| Tingkat pembatalan | Pesanan Dibatalkan atau Kedaluwarsa ÷ pesanan diterima |
| Pesanan ditawar balik atau ditolak | Jumlah dan hasil akhirnya |
| Margin realisasi | Margin per pesanan berdasarkan biaya aktual |
| Hari kas rawan | Jumlah hari proyeksi saldo di bawah buffer |

**Simulasi 30 hari (berlabel SIMULASI):** aliran pesanan sintetis dengan seed tetap dijalankan dua kali: tanpa sistem (semua pesanan diterima) dan dengan sistem (aturan keputusan). Bandingkan jumlah hari kapasitas terlampaui, margin rata-rata, dan hari kas rawan. Karena tidak ada data usaha nyata, hari dengan utilisasi di atas 100% hanya dipakai sebagai indikator berisiko telat, bukan klaim keterlambatan nyata.

## 11. Kebutuhan non-fungsional

| ID | Kebutuhan | P |
|---|---|---|
| NFR-01 | Responsif pada lebar 360, 768, dan 1280 px tanpa scroll horizontal untuk konten utama | M |
| NFR-02 | Diuji di dua versi terbaru Chrome, Firefox, dan Edge (Safari bila memungkinkan); hasil didokumentasikan | M |
| NFR-03 | Status loading, kosong, error, dan sukses pada semua daftar dan aksi tulis | M |
| NFR-04 | Validasi di klien dan server; pesan error spesifik berbahasa Indonesia | M |
| NFR-05 | Keamanan dasar: kata sandi di-hash; hak akses di server; perlindungan XSS, injeksi, dan CSRF; tidak ada .env, token, atau API key di repository; variabel lingkungan didokumentasikan tanpa nilai rahasia | M |
| NFR-06 | Data pelanggan di demo fiktif; hanya data minimum yang dikumpulkan | M |
| NFR-07 | Perubahan lintas modul bersifat atomik (keputusan dan reservasi; pembatalan, pelepasan kapasitas, stok, dan kas); tidak ada data yatim | M |
| NFR-08 | Kalkulasi deterministik dan diuji otomatis terhadap kasus uji Bagian 13 | M |
| NFR-09 | Cek ketersediaan tanggal ≤ 1 detik dan halaman utama ≤ 3 detik pada koneksi umum (H, ambang awal) | S |
| NFR-10 | Website di-deploy publik selama kompetisi, data seed siap, Admin dapat mereset data demo | M |
| NFR-11 | Aksesibilitas dasar: kontras memadai, label form, navigasi keyboard utama | S |
| NFR-12 | Antarmuka berbahasa Indonesia; format Rp, tanggal, dan zona WIB | M |
| NFR-13 | Kode terorganisasi, riwayat commit bermakna, repository publik | M |

## 12. Data demo dan objek data

- Usaha fiktif “Dapur Bu Sari” dengan banner atau footer “Data simulasi”.
- Seed: sekitar 6 produk, 15 bahan, dua sumber daya, 40 pesanan dengan berbagai status, proyeksi kas 14 hari, akun demo tiap peran.
- Skenario demo siap pakai: S1 pesanan besar mendadak (penuh, tawaran balik); S2 permintaan diskon (margin gagal); S3 DP minimal dari model kas; S4 daftar belanja; S5 pembatalan; S6 saran kalibrasi.
- Objek data konseptual (bukan skema): Pengguna, Produk, Bahan, Resep, SumberDaya, Parameter, Pesanan dan ItemPesanan, EvaluasiKeputusan, Pembayaran, Tugas, Pembelian, MutasiStok, EntriKas, AuditLog.

## 13. Kriteria penerimaan (kasus uji berangka)

Kapasitas awal: tenaga 4 × 420 × 0,85 = 1.428 menit; oven 2 × 8 × 0,85 = 13,6 oven-jam. Snack box: 12 menit tenaga dan 0,05 oven-jam; harga normal Rp 25.000; biaya Rp 19.000.

| ID | Kondisi | Hasil yang diharapkan |
|---|---|---|
| TC-01 | Sabtu sudah ada beban tenaga 1.020 menit dan oven 5,0 jam; masuk 100 snack box | Tenaga 2.220 menit (155,5%), oven 10,0 jam (73,5%); hari Penuh; penghambat tenaga; Terima bersyarat |
| TC-02 | Minggu beban awal 60 menit tenaga | Setelah 100 box: 1.260 menit (88,2%), status kuning; Minggu muncul sebagai tawaran balik |
| TC-03 | Harga dinegosiasi Rp 21.000 | Margin 9,5%; harga minimal Rp 23.750; diskon maksimum 5%; G2 gagal |
| TC-04 | Pesanan 100 box @ Rp 23.750 (Rp 2.375.000); belanja Rp 1.300.000 di H-2; saldo awal H-2 Rp 1.800.000; biaya tetap Rp 400.000 per hari; DP pesanan lain Rp 500.000 di H-1; buffer Rp 400.000 | Titik terendah tanpa DP −Rp 200.000; DP minimal Rp 600.000; DP default 30% = Rp 712.500 yang ditagih; titik terendah dengan DP Rp 512.500; G3 lolos |
| TC-05 | Minggu itu 6 tart (0,4 kg tepung) dan 100 box (0,12 kg); stok 3 kg; buffer 10% | Kebutuhan 14,4 kg; dengan buffer 15,84 kg; perlu beli 12,84 kg; dibulatkan 13 kg (eceran) atau 1 kemasan 25 kg sesuai aturan bahan |
| TC-06 | Pesanan TC-04 dibatalkan di H-1 setelah belanja | Eksposur Rp 587.500; kapasitas dilepas; bahan menjadi stok; proyeksi kas dihitung ulang |
| TC-07 | Durasi aktual tart 105, 100, 95, 110, 100 menit; estimasi 90 | Rata-rata 102; deviasi +13,3%; saran koreksi muncul; dengan hanya 4 sampel tidak muncul |
| TC-08 | Dua pesanan hampir bersamaan memperebutkan slot terakhir | Hanya satu Diterima; yang lain mendapat tawaran balik |
| TC-09 | Karyawan membuka halaman kas; pelanggan menebak kode pesanan orang lain | Akses ditolak dengan pesan jelas; data pesanan lain tidak terbuka |
| TC-10 | Tanggal kurang dari 2 hari dari sekarang | Tanggal tidak dapat dipilih; bila lolos manual, G4 gagal dan rekomendasi Tolak |
| TC-11 | DP tidak tercatat sebelum tenggat | Status Kedaluwarsa; kapasitas dilepas |
| TC-12 | Pemilik menerima pesanan yang direkomendasikan Tolak | Alasan wajib diisi; tercatat di audit log dengan nama, waktu, dan rekomendasi awal |

## 14. Kebutuhan dokumentasi dan demo

- **README:** masalah, solusi, fitur, target pengguna, tech stack dan alasannya, arsitektur, cara menjalankan, variabel lingkungan tanpa nilai rahasia, link demo, screenshot, akun demo, dokumentasi API dan skema database bila relevan.
- **Proposal (bagian yang diminta brief):** judul dan nama tim, latar belakang masalah, tujuan dan manfaat, solusi dan target pengguna, arsitektur sistem, diagram database, teknologi beserta alasan, fitur utama, keunikan, batasan, screenshot implementasi, rencana pengembangan.
- **Video demo 3–5 menit:** memperlihatkan alur utama AU-1 dan skenario S1, S3, dan S5, bukan tur halaman.

## 15. Asumsi, risiko, dan hal yang perlu dikonfirmasi

**Asumsi (H):** pemilik usaha kuliner pre-order memang kesulitan menimbang kapasitas, margin, dan kas; beban produksi dapat diestimasi pemilik; DP manual dapat diterima sebagai alur pembayaran; parameter awal pada tabel 8.2 masuk akal.

**Risiko utama:**

- Cakupan terlalu luas: tahan semua Should sampai alur AU-1 berjalan penuh.
- Beban produksi sulit diisi pemilik nyata: sediakan kategori sederhana (ringan, sedang, berat) sebagai alternatif.
- Konsistensi data lintas modul: wajib diuji dengan TC-06 dan TC-08.
- Dampak bisnis hanya simulasi: selalu berlabel.
- Empat dashboard memperbesar pekerjaan UI: batasi tiap dashboard pada maksimal lima elemen.

**Perlu dikonfirmasi ke panitia (tidak disebut jelas di brief):** apakah demo dampak berbasis data simulasi berlabel dapat diterima; format dan tempat penyajian akun demo untuk juri; apakah ada persyaratan platform deployment tertentu (brief hanya menyebut platform publik).

## 16. Pertanyaan terbuka

1. Vertikal akhir: kue pre-order, snack box dan katering, atau keduanya?
2. Beban produksi diisi dengan stopwatch atau kategori sederhana?
3. Apakah pengantaran cukup berupa status, atau perlu daftar antar terurut?
4. Apakah parameter awal sudah diuji ke 2–3 pelaku usaha sebelum dikunci?
5. Mana prioritas bila waktu habis: kalibrasi, simulasi 30 hari, atau notifikasi?

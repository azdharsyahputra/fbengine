# Rencana: Team Shape Controller (satu pemilik bentuk tim)

Status: **draf, menunggu persetujuan**. Belum ada kode yang diubah.

## 0. Kenapa rencana ini ada

Setelah go13–go16, metrik per posisi (giring panjang, CB, FB, penerima bola) membaik sedikit-sedikit, tapi gambarnya masih belum kayak bola beneran. Ada dua pelajaran dari putaran-putaran itu.

### 0.1 Patokan kita sendiri bisa menyesatkan

Kesimpulan awal ("tim numpuk 5×, 13 m lebih sempit") datang dari frame SkillCorner yang 18+ pemainnya terlihat. Frame seperti itu bias: kamera broadcast cuma menangkap semua pemain di momen tertentu. Setelah dicek ulang pakai **Metrica** (2 laga penuh, 22 pemain selalu terlihat, cuma permainan hidup, definisi identik), hasilnya:

| Fase tenang (tim sama pegang bola ≥4 dtk) | go16 | Player12 murni | Rust r53 | **Metrica** |
|---|---|---|---|---|
| Lebar tim pegang bola (y p10–p90) | 36–37 m | 34 | 45 | **37–39** |
| Panjang tim pegang bola | 29–30 m | 38 | 37 | **30** |
| Pasangan setim <4 m, pegang bola | 0.91–0.97 | 0.64 | 0.29 | **0.96–1.78** |
| Jarak teman terdekat, pegang bola | 8.6 m | 10.6 | 12.3 | **10.4** |
| **Panjang blok bertahan** | **35 m** | 27 | 25 | **26–27** |
| Lebar blok bertahan | 29 m | 31 | 29 | **32** |
| Lebar tim pegang bola waktu transisi | 28–30 m | 33 | 34 | **34** |

> **Diperbarui oleh F0** ([F0-baseline.md](F0-baseline.md)): per zona, bentuk menyerang go16 di sepertiga sendiri dan tengah **jauh** dari asli. Lebar 31–36 vs 45 m, jalur pinggir 22–28% vs 59–72%, pivot nempel garis bek, tumpukan di tengah 1.0 vs 0.09. Kalimat "bentuk menyerang tidak jauh dari asli" di bawah ini salah.

Artinya (cek cepat, sebelum F0):
- **Bentuk menyerang go16 secara garis besar tidak jauh dari asli.** Lebar, panjang, dan tumpukan mirip. Pemain cuma sedikit terlalu rapat: jarak teman terdekat 8.6 vs 10.4 m.
- **Gap besar yang terverifikasi ada di blok bertahan:** 35 m vs 26–27 m. Player12 murni masih 27 m, jadi **layer kita sendiri yang bikin blok molor** (kandidat: tugas M1 recover-shape, forward yang jogging pulang sejak go10, press zona, retreat).
- **Lebar waktu transisi menyerang terlalu sempit** (28–30 vs 34 m).
- Rust r53 tidak lebih baik di semua hal. Menyerangnya terlalu lebar (45 m) dan terlalu panjang (37 m).

**Konsekuensi:**
1. Semua target "bentuk" harus diambil dari Metrica, bukan dari frame SkillCorner.
2. Metrik dekat bola (tekanan, penerima, opsi pemegang bola) tetap dari SkillCorner, dengan catatan bias.
3. Target yang dipakai go13–go16 perlu dicek ulang terhadap Metrica (F0).

### 0.2 Arsitektur bikin tiap perbaikan saling tabrak

Posisi satu pemain lewat P12 shape → duty M5 → push/compact M4 → struktur M10 → refine M4 → tugas M1 → filter gerak M3. Ada sekitar 50 `TargetReason` di sekitar 4.000 baris kode. Probe tumpukan nunjukin pasangan yang numpuk berasal dari belasan kombinasi layer. Kebijakan tersembunyi di M3 (batas kecepatan penyesuaian) sempat membatalkan perbaikan FB di go15. Tiap knob baru memperbaiki satu angka dan menggeser yang lain.

**Pembanding:**
- **FootballEngine:** punya `TeamPosition`, satu penentu posisi semua pemain: formasi ala HELIOS + perapian jarak Voronoi (`FieldEvaluator`). Modul ini sengaja tidak kita port.
- **Rust r53:** satu fungsi `targets()` (slot per peran × jalur, lalu prinsip pelatih).

Arah satu pemilik bentuk tetap benar untuk jangka panjang, karena kode bisa dirawat dan perbaikan tidak saling tabrak. **Tapi urutan kerja ditentukan data (F0), bukan asumsi.** Mulai dari gap terbesar yang terverifikasi, yaitu blok bertahan.

## 1. Tujuan dan batasan

**Tujuan:** di fase tenang (pegang bola maupun bertahan), bentuk tim mirip data asli: lebar, jarak antar teman, panjang blok, jarak antar baris, dan kawalan dekat bola. Perilaku khusus (lari, overlap, press, duel, bola liar) tetap ada, tapi jadi permintaan yang diatur.

**Definisi selesai:**
- semua gerbang metrik bentuk (§3) lolos di 10 seed;
- metrik lama tidak mundur melewati toleransi (§3.3);
- replay ditonton dan lolos review empat situasi tetap (§6).

**Bukan tujuan rencana ini:**
- Kalibrasi statistik (tembakan, foul, umpan mundur, penerima bola). Itu fase terakhir (F5), setelah bentuk benar.
- Mengubah M2 (keputusan), M6 (duel), M9 (bola mati), atau kiper, kecuali yang perlu untuk antarmuka.
- Mode liga lama tetap identik.

**Aturan tetap:**
- Determinisme dan golden hash.
- `--reproduce`.
- Jalur lama bisa dipilih lewat config sampai jalur baru jadi default.
- Referensi GPL boleh di-port dengan atribusi.
- Engine CM/FM tetap clean-room.
- Data pemain dan referensi tidak di-commit.

## 2. Patokan (ground truth)

| Sumber | Isi | Dipakai untuk | Batasan |
|---|---|---|---|
| **Metrica Sports sample** (`refdata/metrica`, 2 laga penuh, 25 Hz, 22 pemain selalu terlihat, ada event) | Posisi penuh tanpa bias kamera | **Patokan utama bentuk tim**: lebar, panjang, baris, tumpukan, tabel slot per peran | Anonim, level liga tidak diketahui, cuma 2 laga. Peran harus ditebak dari posisi rata-rata. |
| SkillCorner 2020 (4 laga EPL/LaLiga) | Broadcast tracking, pemain dekat bola | Metrik dekat bola: tekanan, penerima, opsi pemegang bola, distribusi penerima | Jauh dari bola sering tidak terlihat. Frame ≥18 terlihat cuma sekitar 7 menit total. |
| Player12 murni, Rust r53 | Baseline engine | Pembanding, bukan target | — |

**Catatan:**
- Angka Metrica di §0.1 dari cek cepat: live mask dan penguasaan masih kasar, peran belum ditebak.
- F0 membuat versi rapi: konversi ke replay, live mask dari event, peran per pemain, dan metrik per zona bola.
- Frame SkillCorner ≥18 terlihat **tidak dipakai** untuk target bentuk.

## 3. Paket metrik bentuk (dibangun di F0, dipakai di semua fase)

### 3.1 Definisi fase (dari replay)

- **Pegang bola tenang:** tim yang sama memegang bola ≥4 detik dan bola tidak sedang dalam umpan panjang.
- **Bertahan tenang:** kebalikan dari pegang bola tenang.
- **Transisi:** 0–4 detik setelah pergantian penguasaan.
- **Bola liar / di udara:** dipisah.
- Semua metrik juga dipecah per zona bola (sepertiga sendiri / tengah / akhir).

### 3.2 Metrik dan target

Target diisi dari Metrica di F0, toleransi ±1 sd antar-laga.

| # | Metrik | Fase |
|---|---|---|
| S1 | Lebar tim (y p10–p90) | Pegang / bertahan |
| S2 | Panjang tim (x p10–p90) | Pegang / bertahan |
| S3 | Pasangan setim <4 m per frame; jarak teman terdekat (median) | Keduanya |
| S4 | Jarak antar baris: bek–DM, DM–AM, AM–ST | Keduanya |
| S5 | Keterisian jalur: 5 jalur lebar; % frame dengan kedua jalur pinggir terisi | Pegang |
| S6 | Tinggi garis belakang relatif bola dan absolut | Keduanya |
| S7 | Kawalan: bek dalam 3 m / 5 m dari penyerang; jarak penanda ke penerima | Bertahan |
| S8 | Geser ke sisi bola: titik tengah y tim vs y bola | Bertahan |
| S9 | Opsi pemegang bola: jarak dan dx teman per peran | Pegang (SkillCorner) |
| S10 | Distribusi penerima per peran per zona | Pegang (SkillCorner + Metrica) |

### 3.3 Metrik lama yang dijaga

Toleransi terhadap go16, 10 seed:
- chase≥3, liar5, ping-pong engine;
- carry ≥25 m;
- tekanan build-up (own3near / in5);
- mid3in5;
- recvPress, kept;
- umpan mundur per zona;
- box free;
- gol, foul.

Boleh membaik, tidak boleh mundur lebih dari sekitar 15%.

## 4. Arsitektur target

```
                 ┌──────────────────────────────┐
 tactic, lineup, │ TEAM SHAPE CONTROLLER (TSC)  │  satu pemilik bentuk, per tim, per refresh
 ball anchor,    │  formation table + interp.   │
 phase  ───────► │  attacking / defensive rules │──► 10 slot (team frame) + assignment
                 │  slot assignment (hysteresis)│
                 └──────────────┬───────────────┘
                                │ slots
 behaviour modules ──requests──►│
 (P12 jobs, M1, M5, runner,     ▼
  overlap, press, loose, ...) ┌──────────────────────────────┐
                              │ POSITION COMPOSER            │  arbitrase: prioritas, kuota,
                              │  slot + permitted offset     │  batas deviasi, jarak minimal,
                              │  final spacing pass          │  kelas kecepatan
                              └──────────────┬───────────────┘
                                             ▼ target + speed class
                              ┌──────────────────────────────┐
                              │ M3 MOVEMENT (executor only)  │  steering, accel, separation fisik
                              └──────────────────────────────┘
```

### 4.1 Team Shape Controller (`engine/team_shape.go`, baru)

1. **Tabel formasi:**
   - Bentuknya slot per peran (GK, CB×2, FB×2, DM×2, AM-L/C/R, ST untuk 4-2-3-1; peran lain dipetakan per kelompok) di titik jangkar bola. Ada jangkar terpisah untuk fase pegang bola dan bertahan.
   - Interpolasi pakai inverse-distance antar jangkar, mirip `Formation.fs` FE dengan atribusi.
   - **Nilainya dikalibrasi dari Metrica:** posisi median tiap peran relatif ke bola per zona bola dan fase.
   - Slider taktik (lebar, tinggi blok, kerapatan) jadi geseran yang terdefinisi, bukan knob tersebar.
2. **Aturan bentuk menyerang:**
   - 5 jalur referensi, dan dua jalur pinggir wajib terisi (winger atau FB).
   - Kedalaman variabel per baris; ST paling depan dengan batas onside; garis belakang naik dengan batas tinggi.
   - FB sisi bola ikut di baris tengah; FB sisi jauh jadi bagian rest defence.
3. **Aturan bentuk bertahan:**
   - Dua baris (4+4 atau sesuai formasi), dengan tinggi blok dari taktik.
   - Batas panjang (S2) dan lebar (S1), geser ke sisi bola (S8).
   - Jarak antar pemain satu baris dijaga (berlaku juga ke jarak antar CB, dulu `m1.rest_gap`).
4. **Penugasan pemain ke slot:** greedy dengan histeresis, peran utama diprioritaskan. Tujuannya pemain ga tuker-tukeran slot tiap tick.
5. **Perapian jarak:** mirip `FieldEvaluator` FE. Kandidat di sekitar slot dinilai dari jarak ke lawan, jarak minimal ke teman, dan progres. Cuma berlaku buat pemain yang memegang slot (bukan job bola).

### 4.2 Position Composer (`engine/compose.go`, menggantikan `compose_attack.go`)

**Permintaan** = {pemain, jenis, titik, prioritas, deviasi maksimal dari slot, kelas kecepatan, kadaluarsa, owner/reason}.

**Aturan arbitrase, berurutan:**
1. **Job bola tanpa batas deviasi:** pembawa, penerima umpan, perebut bola liar, first defender/contain, penekan aktif.
2. **Job berkuota:** pelari ke belakang ≤2, overlap ≤1, pelari ke kotak ≤2, penekan tambahan ≤1. Kuota dan pemilihan pemain diputuskan composer, bukan modulnya.
3. **Offset terbatas:** support dekat bola, cover, penanda, duty M5. Hasilnya slot + offset, dipotong ke deviasi maksimal.
4. **Sisanya** pakai slot (sudah dirapikan TSC).
5. **Jarak minimal akhir** (sekitar 8 m, dari F0) antar teman, kecuali tingkat 1.

**Kelas kecepatan:** jalan / jogging / lari / sprint, diputuskan composer per permintaan. Ini menggantikan kebijakan tersembunyi di M3.

**Observabilitas:**
- `TargetOwner` / `TargetReason` tetap, ditambah `SlotDeviation`.
- Probe baru: deviasi dari slot per jenis job, kuota terpakai, pelanggaran jarak.

### 4.3 M3 jadi eksekutor saja

Dipertahankan: fisika, akselerasi, belokan, separation fisik, reaksi dan antisipasi pribadi.

Dibuang atau dipindah ke composer:
- batas kecepatan penyesuaian (0.9 + 0.2 m/s per meter);
- kebijakan catch-up dan settled share;
- filter unit bek;
- deadband per kelas;
- aturan transisi jogging.

Pemicunya: kebijakan tersembunyi di M3 yang membatalkan perbaikan go15.

### 4.4 Nasib modul lama

| Sekarang | Nanti |
|---|---|
| P12 blok shape + holder shape (`b2_tactical_targets.go`) | Diganti tabel formasi TSC |
| P12 support near/square/trailing, short-forward | Permintaan support (tingkat 3) |
| P12 runner, arrival, far-post, overlap | Permintaan berkuota (tingkat 2) |
| P12 carrier, receive, flight-*, transition-* | Job bola / transisi (tingkat 1–3) |
| M1 (first defender, cover, contain, screen, marker, protect depth, recover shape) | First defender/contain di tingkat 1; cover dan marker di tingkat 3; protect depth dan recover shape **dihapus** (jadi bentuk bertahan TSC) |
| Press zona (C3) | Permintaan penekan + aturan tinggi blok |
| M4 shape anchor | Input TSC (jangkar bola terprediksi), tetap |
| M4 push FB, compaction, refine | **Dihapus**, diganti aturan bentuk + perapian jarak TSC |
| M5 duty | Permintaan offset (tingkat 3) |
| M10 struktur, get-open | **Dihapus**, diganti aturan jalur TSC dan perapian jarak |
| Loose resolver | Tetap, jadi job bola |
| M9 bola mati, kiper | Tetap (di luar rencana) |
| Knob go13–go16 (rg, fc, cr, ol, ft/fw/fz, ps/pq, cw, dll.) | Dibuang saat jalur lama dihapus (F4) |

## 5. Fase

> **Diubah setelah F0 (keputusan lu):** pegang bola, bertahan, dan transisi **tidak dikerjakan satu per satu.** Dikerjakan satu per satu, tiap bagian disetel terhadap bentuk lawan yang lama, dan kita balik ke pola go13–go16 (satu angka beres, yang lain geser). Jadi TSC dibangun sebagai **satu sistem terorkestrasi untuk kedua tim di semua fase**, dan dinilai pakai **satu scorecard gabungan**.

### 5.1 Aturan orkestrasi

1. **Satu pengatur, dua tim, semua fase.** Tiap refresh, TSC menghitung slot untuk kedua tim sekaligus dari tabel formasi yang sama:
   - pegang bola: tabel fase `att`;
   - bertahan: tabel fase `def`;
   - transisi: perpindahan halus dari slot `att` ke `def` (atau sebaliknya) selama sekitar 4 detik, dengan pemain dekat bola tetap di job bola.

   Bentuk pegang bola tim A dan bentuk bertahan tim B selalu dihitung bersamaan, jadi tidak ada bagian yang disetel terhadap bentuk lama.
2. **Satu composer** untuk semua permintaan dari dua tim, dengan aturan prioritas, kuota, dan jarak yang sama.
3. **Satu scorecard gabungan** (`audit/scorecard.js`):
   - Semua metrik bentuk (S1–S8, penerima, kecepatan) untuk pegang bola, bertahan, dan transisi di tiap zona dinilai **sekaligus** terhadap rentang Metrica.
   - Metrik lama (§3.3) ikut dinilai sebagai penjaga.
   - Hasilnya satu skor dan daftar lolos/gagal.
   - **Satu perubahan hanya diterima kalau skor total membaik dan tidak ada penjaga yang jebol.** Ga boleh lagi "angka X beres, Y geser".
4. **Yang boleh disetel cuma parameter TSC/composer** (tabel formasi, batas blok, jarak minimal, kuota, kelas kecepatan). Disapu bersama dan dinilai pakai scorecard yang sama. Knob per gejala di modul lain dilarang.
5. **Statistik (tembakan, foul, dll.) tidak disentuh** sampai scorecard bentuk lolos (F4).

**Penjaga dipisah (2026-11-24, keputusan lu):**

| Kelompok | Metrik | Jadi gerbang |
|---|---|---|
| Penjaga bentuk | chase≥3, bola liar ≥5, DM–AM, tekanan build-up (own3in5), tekanan tengah (mid3in5), penerima ditekan (recvPress) | F2/F3 |
| Penjaga keputusan | umpan mundur, bola dijaga (kept), umpan mundur di sepertiga akhir/tengah, lama penguasaan (possMed) | F5 (kalibrasi M2 di atas bentuk baru) |

`scorecard.js --guard` melaporkan keduanya terpisah.

### 5.2 Tahapan

Tahapan ini urutan **bangun**, bukan urutan **nyalakan**. Jalur baru dinyalakan dan dinilai sebagai satu kesatuan. Jalur lama (go16) tetap identik byte selama `Modules.TeamShape = false`.

| Tahap | Isi | Gerbang |
|---|---|---|
| **F0: patokan dan alat ukur** (selesai, `f05e79d`) | Konverter Metrica, shape suite, tabel formasi, baseline, audit ulang target lama | Lihat [F0-baseline.md](F0-baseline.md) |
| **F1: TSC + composer, mode bayangan** (selesai, lihat [F1-shadow.md](F1-shadow.md)) | TSC (pegang bola + bertahan + transisi, dua tim) menghitung slot tiap refresh tanpa mengubah gerak. Composer dan tipe permintaan dibangun, tapi belum memutuskan. Probe: slot vs posisi aktual. Viewer: gambar slot. Scorecard dibuat. | Scorecard **slot** (posisi kalau semua pemain berdiri tepat di slot) lolos untuk **semua** fase dan zona. go16 identik byte saat off. |
| **F2: nyalakan semuanya bersamaan** | `TeamShape` on: posisi tenang dari TSC, M1/P12/M5/M10 jadi permintaan, transisi dari aturan TSC. Kelas kecepatan dari composer (M3 cuma eksekutor). | Scorecard gabungan **aktual** (pemain bergerak, ada duel, ada bola) lolos semua fase. Review visual empat situasi lolos. Penjaga tidak jebol. |
| **F3: sapu parameter bersama** | Kalau F2 belum lolos penuh: sapu parameter TSC/composer **bareng-bareng** (bukan per metrik), pilih pakai skor total scorecard. | Scorecard lolos. Hasil sapuan 10 seed. |
| **F4: hapus jalur lama** | `TeamShape` jadi default. Kode dan knob lama (go13–go16) dihapus. go16 tetap bisa dijalankan dari git tag. | Tes, golden hash baru, `--reproduce`. |
| **F5: kalibrasi keputusan dan statistik** | M2/M6 disetel ulang di atas bentuk baru: tembakan, foul, umpan mundur, penerima, penguasaan bola, giring panjang. Scorecard bentuk tetap jadi penjaga. | Statistik ±20% dari data asli **dan** scorecard bentuk tetap lolos. |

## 6. Protokol validasi tiap fase

1. **Otomatis:** `audit/eval.sh` 10 seed + `audit/scorecard.js` (skor total + daftar lolos/gagal semua fase sekaligus).
2. **Review visual tetap:** empat situasi, filmstrip dari atas (`draw.py`) dibandingkan Metrica pada situasi serupa, plus tonton di viewer:
   - build-up dari belakang;
   - serangan sayap di sepertiga akhir;
   - bertahan blok rendah;
   - serangan balik.
3. **Laporan:** apa yang lolos, apa yang tidak, apa yang dicoba dan dibuang. Commit hanya kalau gerbang lolos atau ada keputusan sadar dari lu.

## 7. Risiko dan mitigasi

| Risiko | Mitigasi |
|---|---|
| Perubahan besar sekaligus bikin engine rusak lama | Mode bayangan (F1) menilai slot dulu sebelum dinyalakan. Flag modul. Jalur lama identik byte. |
| Metrica cuma 2 laga, level tidak diketahui (kemungkinan bukan liga top) | Cocokkan metrik dekat bola dengan SkillCorner (EPL/LaLiga). Pakai rentang, bukan titik. |
| Formasi asli beda (4-3-3, 4-4-2) dengan label 4-2-3-1 kita | Tabel per kelompok peran (bek tengah, bek sayap, gelandang bertahan, sayap, tengah, depan), bukan per formasi |
| Kalibrasi M2/M1 lama bergantung bentuk lama (duel_loss, progress, contain window) | Diterima. Dikalibrasi ulang di F5, tidak dikejar sebelum scorecard bentuk lolos. |
| Performa (penugasan + perapian jarak tiap refresh, RAM 8 GB) | Greedy + histeresis, kandidat perapian sedikit (FE: sekitar 6), separuh pemain per refresh (cara FE) |
| Kuota/prioritas bikin pemain kaku | Kuota dan deviasi jadi config, disapu di tiap fase, dicek visual |
| Tergoda nambal lagi | Aturan: tidak ada knob baru di luar TSC/composer. Masalah bentuk diselesaikan di TSC. |

## 8. Keputusan

Sudah diputuskan:
- Patokan bentuk = Metrica.
- F0 dulu.
- Pegang bola, bertahan, dan transisi dibangun dan dinyalakan **bersamaan** dengan satu scorecard gabungan.

Masih terbuka:
1. **Sumber tabel formasi:** dari Metrica dengan interpolasi ala FE (saran), atau port formasi FE apa adanya.
2. **Kerja paralel:** sesi engine Rust lagi idle. Mau dipakai juga sebagai pembanding?

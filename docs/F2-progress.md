# F2: Team Shape Controller + composer menyala (progres, 2026-11-24)

Bagian dari [PLAN-team-shape.md](PLAN-team-shape.md). **Checkpoint:** `Modules.TeamShape` masih default **mati**, jadi go16 identik byte. Nyalakan dengan `FMENGINE_ON=TS`. Parameter composer ada di `EngineConfig.TeamShape`, dengan override lewat key di `FMENGINE_M1`.

## Wiring (`engine/team_shape_compose.go`)

**Pegang bola.** Keputusan layer lama (P12, M4/M5/M10, loose resolver) dibaca sebagai permintaan, menurut reason/intent-nya:

| Tingkat | Isi | Perlakuan |
|---|---|---|
| Job bola | carry, receive, claim, press | Target tetap |
| Berkuota | lari ke belakang, ke kotak, overlap, doubling | Diberi per tim sampai kuota |
| Offset | support, duty | Dijepit dekat slot |
| Lainnya | — | Slot (termasuk flight-support: selama umpan di udara semua lanjut ke slot) |

**Bertahan, native (`teamShapeDefence`, tidak memakai tugas M1).** Semua pemain pegang slot, kecuali:
- **1 first defender:** paling cepat sampai 2 m di sisi gawang pembawa bola.
  - Jendela 4 dtk, di zona press 6 dtk; dekat gawang selalu.
  - Histeresis; kiper lawan tidak dikejar.
- **1 cover:** 6 m di belakang first defender.
- **Di zona press, 1 penekan kedua** yang menutup opsi umpan terdekat.
- **Penanda berorientasi bola:**
  - Penyerang dalam 25 m dari bola (atau di zona bahaya saat bola dekat gawang) ditandai 2 m di sisi gawang oleh pemegang slot terdekat (≤12 m dari slotnya).
  - Di zona bahaya penanda sprint; di luar itu bergerak secepat orangnya.
- **Garis belakang** turun hanya bersama pelari yang benar-benar lari ke belakang (>3.5 m/s).
- **Bola liar / umpan di udara:** job perebutan bola tetap.

**Gerak (M3 sebagai eksekutor).** Target composer diikuti langsung (tanpa filter, deadband, reaksi tertunda). Kecepatan = kecepatan target + menutup jarak dalam 2.5 dtk:
- tim bertahan geser maks 2.4 m/s (lari hanya kalau >12 m dari slot, maks 4.5);
- tim pegang bola jalan saat ≤3 m dari slot;
- pemain bertahan yang terlewati bola pulang sambil trot 2.6 m/s.

**Jangkar bentuk.** Tim bertahan mengikuti bola yang dihaluskan (3 dtk); tim pegang bola mengikuti bola langsung.

## Hasil (10 seed, `FMENGINE_ON=TS`)

| | go16 | F2 sekarang | Asli |
|---|---|---|---|
| **Scorecard** (makin kecil makin baik) | 1.41 | **0.99** | — |
| Lolos | 57/135 | **85/135** | — |
| Penjaga | 4/11 | 4/11 | 11/11 |
| Lebar pegang bola (sendiri / tengah) | 31.5 / 36.2 | 42.6 / 42.9 | 44.8 / 45.6 |
| Jalur pinggir terisi (tengah) | 28% | 43% | 72% |
| Panjang blok bertahan (tengah) | 35.2 | **26.5** | 24.1 |
| Bek–DM / DM–AM bertahan (tengah) | 12.2 / 13.2 | 10.2 / 6.9 | 8.7 / 7.3 |
| Umpan mundur / tekanan tengah | gagal | lolos | — |
| Jalan <2 m/s (% waktu) | 30 | 32 | 48 |

**Penjaga yang masih gagal:**
- chase≥3: 24 (asli 5–10)
- bola liar ≥5: 26 (asli 6–10.5)
- tekanan build-up lawan: 17% (asli 36–50). DM lawan bebas (asli 65% ditekan); kiper sudah tidak dikejar.
- penerima ditekan: 21% (asli 24–37)
- bola dijaga: 89% (asli 74–86)
- penguasaan bola: 9.8 dtk (asli 4.5–7.5)
- DM–AM: 10.8 (batas 10.5)

## Pelajaran dari putaran F2

1. **Pertahanan lewat tugas M1 = bentuk lama bocor.**
   - Membaca tugas M1 sebagai offset menarik pemain ke blok lama yang panjang. Blok mentok 34–35 m di semua sapuan.
   - Versi native langsung membawa blok ke 26.5 m.
2. **Kecepatan bukan tuasnya.** Sapuan kecepatan dan jangkar (32 kombinasi) hanya menukar chase dengan kebobolan.
3. **Penanda dekat bola:**
   - sprint ⇒ chase tinggi;
   - geser santai ⇒ kebobolan;
   - secepat orangnya ⇒ seimbang.
4. **Tekanan build-up salah sasaran:** kiper dikejar, DM lawan dibiarkan.

## Putaran 2 (2026-11-24): penjaga yang tersisa (semua 10 seed, dibanding checkpoint `1ba367d`)

**Temuan:**
- **Tempo:** tim pegang bola terlalu banyak lari.
  - Lebih dari 4 m/s: 43% waktu (Metrica 12%); jalan 22% (Metrica 47–51%).
  - Penanda yang bergerak secepat orangnya ikut membawa tim bertahan berlari, sehingga chase naik.
- **Pelari chase:**
  - sebagian besar penanda dan pemegang slot yang jauh (>12 m) dari slotnya (transisi);
  - first defender cuma 6%.
- **Tekanan build-up lawan (Metrica vs kita):**
  - DM lawan ditekan 65% vs 19–20%;
  - CB 24–32% vs 6–10%;
  - kiper 19–21% vs 57–64%. Kiper sekarang sudah tidak dikejar (5–6%).

**Yang dicoba:**

| Varian | Skor | Lolos | Penjaga | chase | Bola liar | Giring ≥25 | Gol |
|---|---|---|---|---|---|---|---|
| Checkpoint | **0.99** | 85 | 4 | 24 | 26 | **28** | **3.1** |
| + penanda wajib di sisi gawang, batas tempo pegang bola 3 m/s, zona jalan 9 m, first defender jockey | 1.15 | 72 | **6** | 19 | 21 | 37 | 4.9 |
| sama, tanpa batas tempo pegang bola | 1.09 | 82 | 5 | 20 | 23 | 38 | 4.6 |
| sama, tanpa jockey | 1.08 | 78 | 5 | 18 | 20 | 36 | 3.7 |
| sama, tanpa jockey dan tanpa syarat sisi gawang | 0.97 | **88** | 3 | 20 | 19 | 36 | 4.0 |

**Keputusan:** tidak ada varian yang lebih baik dari checkpoint di skor **dan** penjaga **dan** gol sekaligus. Semuanya dibuang, kode tetap di checkpoint.

**Pelajaran:**
1. Hasil 3 seed terlalu berisik untuk keputusan (gol 2–6, skor ±0.1). Keputusan hanya dari 10 seed.
2. chase≥3 dan bola liar ≥5 tidak turun di bawah ~18–20 dengan aturan kecepatan apa pun, dan tiap penurunan dibayar gol atau giring panjang.
3. Sebagian penjaga lama adalah urusan keputusan (M2), bukan bentuk: penguasaan bola (≈10 dtk vs 4.5–7.5) dan bola dijaga (87–89% vs 74–86%). Itu tahap kalibrasi keputusan (F5).

## Putaran 3 (2026-11-24): tekanan build-up (10 seed)

**Temuan (Metrica):** pembawa bola di sepertiga sendiri ditekan dalam 5 m:
- 51–56% sesaat setelah merebut bola (counter-press);
- 32–34% saat tenang.

Kita: 22–25% / 12–14%. First defender selalu **mulai dari diam**, karena baru ditunjuk setelah bola dikontrol (kecepatan rata-rata saat masih jauh 4.5 m/s).

**Yang tidak berpengaruh:**
- histeresis first defender per pembawa bola;
- counter-press 4 dtk;
- penandaan 40 m dari bola di zona press.

Ketiganya tetap di kode: perlu bersama pressing trigger, dan murah.

**Kuncinya: pressing trigger** (`team_shape.pass_trigger`). Selama umpan lawan di udara, first defender langsung menuju penerima yang dituju (`IntendedReceiver`), jadi berangkat bersama umpan.

| | Checkpoint | + pressing trigger | Asli |
|---|---|---|---|
| Skor | 0.99 | **0.907** | — |
| Lolos | 85/135 | **93/135** | — |
| Penjaga bentuk | 1/6 | 2/6 | — |
| own3in5 | 17 | **37** ✓ | 36–50 |
| recvPress | 21 | **31** ✓ | 24–37 |
| Ditekan setelah merebut / tenang | 22 / 13% | **45 / 35%** | 51–56 / 32–34% |
| mid3in5 | 43 | 57 | 42–54 (sedikit lewat) |
| Giring ≥25 m (probe) | 28 | **17** | — |
| Gol | 3.1 | 3.5 | — |
| Foul | 22 | 27 | — |

Sisa penjaga bentuk yang besar: chase≥3 (23) dan bola liar ≥5 (27).

## Putaran 4 (2026-11-24): bola liar dan chase (10 seed)

- **Composer tidak jalan selama fase bola mati.** `setPiecePhaseRemaining` baru habis setelah tim bertahan mendapat bola, jadi setelah sepak pojok atau tendangan bebas seluruh tim kembali ke logika P12 lama dan mengerumuni bola. Sekarang composer jalan juga di fase itu. Pengatur bola mati (aerial-hold, restart) tetap di job lamanya.
- **Bola benar-benar liar** (tanpa pemegang, bukan umpan): penanda tidak ikut orangnya; perebutan diurus resolver.
- **Intent rebut/tekan** dihitung job bola hanya untuk perebut dan penekan resmi. Tidak ada efek terukur, tapi tetap dipakai karena aturannya lebih tepat.

| | F2b | F2c | Asli |
|---|---|---|---|
| Skor | 0.907 | **0.852** | — |
| Lolos | 93/135 | **95/135** | — |
| Probe bola liar ≥5 | 18.9 | **11.3** | — |
| cek bola liar ≥5 | 27 | 25 | 6–10.5 |
| chase≥3 | 23 | 23 | 5–10 |
| Gol | 3.5 | **2.5** | — |
| Penyerang bebas di kotak | 24 | 39 | — |

**Syarat penanda dari sisi gawang** (`mark_behind` 3) dites lagi: gol 5.1. Tetap mati (99).

## Putaran 5 (2026-11-24): bedah chase≥3 dan bola liar ≥5

**Definisi cek** (`viewer/cek/chase.js`):
- **chase≥3:** bola dipegang (ada pemain dalam 0.8 m) dan ≥3 pemain bertahan lari di atas 3 m/s lurus ke bola dari jarak lebih dari 5 m.
- **"Bola liar ≥5":** setiap momen **tanpa** pemain dalam 0.8 m dari bola. Ini termasuk semua umpan di udara, bukan cuma bola liar beneran.

**Profil pelari** (bola dipegang):

| | Metrica | F2c | go16 |
|---|---|---|---|
| Pelari bertahan per saat | 0.65–0.86 | **1.5–1.8** | 0.72 |
| Jarak ke bola (median) | 16–21 m | 20–22 m | 12 m |
| Kecepatan (median) | 3.8 m/s | 4.7 m/s | 4.8 m/s |
| Sedang mengikuti orang | 30–36% | 26% | 15% |
| Sudah terlewati bola | 43–48% | 39–40% | 21% |

Arah dan konteks lari F2c **mirip asli**. Bedanya jumlah (2×) dan kecepatan.

**Tempo bola mirip asli:**

| | Metrica | Engine |
|---|---|---|
| Pergantian pemegang per menit hidup | 19–20 | 21–25 |
| Lama tanpa pemegang (median) | 1.0 dtk | 1.0 dtk |
| Kecepatan bola tanpa pemegang (median) | 10–11 m/s | 11–12 m/s |

**Akar masalahnya fisika dan kebijakan gerak** (sudah ada di go16):

| | Metrica | F2c | go16 |
|---|---|---|---|
| Percepatan p50 / p90 (m/s²) | 0.75 / 2.0 | 1.24 / 4.6 | 0.99 / 4.2 |
| Lari >4 m/s per pemain-jam | 122–127 | 345 | 311 |

Pemain mengegas dan mengerem dengan tenaga penuh untuk setiap koreksi kecil. Probe baru `AUDIT fast (>4 m/s)` di F2c (32.8% langkah pemain):
- slot tim pegang bola 10.3%
- penanda 7.7%
- first defender 2.3%
- job lama (support, carry, runner) sisanya

**Dicoba dan tidak membantu** (10 seed, dibuang):

| Yang dicoba | Hasil |
|---|---|
| Porsi feed-forward 0.5 | chase 23 |
| Jangkar dihaluskan untuk tim pegang bola | chase 21, gol 3.9 |
| Batas kecepatan penanda 4 m/s | chase 24 |
| Porsi tenaga 0.25–0.5 untuk gerak slot/offset | Percepatan p90 3.3–4.2, tapi lari >4 m/s tetap ~300/jam dan chase **naik** ke 25–31 (pemain yang lambat mengegas jadi telat lalu lari lagi) |
| Radius penandaan 15–25 m | chase 21–24 |

**Kesimpulan:** chase dan bola liar adalah **gejala intensitas gerak**, bukan aturan posisi. Tenaga penuh juga dipakai job lama yang belum lewat composer (penerima, pelari, pembawa bola, first defender), dan kebijakan kecepatan composer belum dikalibrasi ke profil gerak asli.

**Usulan langkah:** kalibrasi gerak sebagai satu kesatuan.
1. Semua job (termasuk job lama) mendapat kebijakan kecepatan dan tenaga dari composer. M3 hanya eksekutor.
2. Kebijakan itu dikalibrasi ke Metrica:
   - porsi kecepatan;
   - lari >4 m/s per pemain-jam;
   - percepatan p50/p90.
3. chase dan bola liar dinilai sebagai akibatnya.

## Putaran 6 (2026-11-24): kalibrasi gerak (10 seed)

**Ukuran baru di scorecard** (`shape_suite` MOVE):

| | Metrica | F2c | go16 |
|---|---|---|---|
| Percepatan p50 / p90 (m/s²) | 0.75 / 2.05 | 1.36 / 4.68 | — |
| Lari >4 m/s per pemain-jam hidup | 234 | 681 | — |
| Skor dengan metrik gerak | — | 0.921 | 1.451 |

**Probe pemicu sprint** (`AUDIT sprint starts`):
- F2c: 1.007 awal sprint per pemain-jam; target lompat >4 m 2.148 per pemain-jam, 81% karena job berganti.
- go16: 670 / 2.319.

| Yang dicoba | Hasil |
|---|---|
| Pasangan penanda dipertahankan + durasi job minimum | Lompatan target turun sedikit, sprint tidak |
| Penghalusan target 0.6 / 1.2 dtk | Lompatan 2.148 → 1.320, sprint 649 / 580 per jam hidup, skor memburuk (0.97 / 0.99) |
| Skala percepatan dan rem global 0.6 / 0.45 (semua pemain) | Percepatan p90 3.54 / 2.95, tapi sprint tetap 589 / 529, porsi >4 m/s tetap ~30%, chase **naik** ke 27–31 |

**Kesimpulan:** yang terlalu tinggi adalah **kecepatan yang diminta** tiap job, bukan percepatan:
- support, penerima, dan transisi (jalur lama);
- first defender, penanda, dan slot yang jauh (composer).

Pemain yang mengegas lebih pelan tetap mencapai >4 m/s, hanya terlambat. Semua knob gerak di atas dibuang. Yang disimpan: metrik gerak di scorecard dan probe pemicu sprint.

**Langkah berikutnya:** tabel kebijakan kecepatan per job dan fase untuk **semua** job (termasuk jalur lama), dikalibrasi ke distribusi kecepatan per peran dan fase di Metrica. M3 cukup mengeksekusi.

### Ronde 7: tabel kecepatan per peran dan fase (satu uji, stop rule)

`audit/speed_table.js` mengukur kecepatan Metrica per kelompok peran (CB, FB, DM/CM, AM/W, ST) × fase (menyerang/bertahan × settled/transisi) dan menghasilkan `engine/team_shape_speed.go` (P80 dan P90). Flag `SpeedTable` (kunci `sq`, default off): slot/offset/job native tidak mendesak dalam RunDistance dibatasi P80, penanda non-bahaya dibatasi P90. Flag off = output TS identik F2c.

| 10 seed | Skor | chase3 | liar5 | gol | lari >4 m/s per jam | porsi >4 m/s |
|---|---|---|---|---|---|---|
| F2c | 0.921 | 23 | 25 | 2.5 | 681 | ~30% |
| A `sq=1` | 0.949 | 23 | 23 | 2.4 | 499 | 21% |
| B `sq=1,rd=99` | 1.041 | 16.5 | 23 | 3.9 | 361 | 15% |

**Hasil: gagal stop rule.** Kecepatan turun jauh (B mendekati Metrica: porsi >4 m/s 15% vs 13%), tapi chase3/liar5 hampir tidak bergerak dan skor total memburuk.

**Kesimpulan penting:** chase dan bola liar **bukan** masalah kecepatan. Pemain yang dilambatkan tetap menuju bola, hanya lebih lambat. Masalahnya **siapa yang diberi job menuju bola** (jumlah pelari), bukan seberapa cepat mereka lari. Tabel disimpan sebagai alat (default off).

Sesuai stop-loss: tuning gerak dihentikan.

## Berikutnya

1. **Pembedahan pelari di momen chase dan bola liar** dalam mode native, lalu aturan composer siapa dan berapa orang yang boleh menuju bola.
2. **Penanda build-up untuk pivot lawan:** slot penyerang kita di zona press dekat DM lawan.
3. **Sapuan bersama parameter composer** dengan scorecard + penjaga.
4. **F4 (hapus jalur lama)** setelah scorecard dan penjaga lolos.

# F5: kalibrasi keputusan dan statistik

TeamShape default sejak `0757c8b`. Statistik dan keputusan disetel di atas bentuk baru; scorecard bentuk tetap jadi penjaga.

## Baseline F5 (10 seed, per tim, dibandingkan EPL 23/24)

| Statistik | F2c | go16 | EPL |
|---|---|---|---|
| Gol per laga | 2.5 | 2.9 | 3.28 |
| Tembakan | 19.3 | 18.4 | ~13 |
| Tepat sasaran | 5.5 | 4.5 | ~4.5 |
| Foul | 26.5 | 17.5 | ~10.5 |
| Kuning per laga | 8.4 | 5.5 | ~4.2 |
| Merah per laga | 1.5 | 0.2 | ~0.15 |
| Corner | 9.3 | 9.4 | ~5.3 |
| Offside | 10.0 | 5.1 | ~1.8 |

## Ronde 1: foul

Probe `AUDIT fouls` (FMENGINE_AUDIT=1) menghitung foul per jenis, sanksi, job pelaku, dan hasil tiap challenge M6 per jenis (standing/sliding).

**Akar:** jumlah percobaan tekel naik, bukan rasio foul per tekel.
- TS 228 percobaan per laga, jalur lama 167; rasio foul sama (~20%).
- 61% tekel TS adalah sliding dari bek yang mengejar dari belakang. Sliding meleset 50% dan menang 20%; standing menang 46% dan meleset 8%.

**Perubahan:** `M6.SlideDepth` (kunci `gz`). Bek yang mengejar dari belakang hanya boleh sliding saat pembawa bola dalam 35 m dari gawangnya sendiri. Di luar itu dia terus mengejar untuk standing tackle.

| 10 seed | Skor | Foul/tim | Merah/laga | Gol | chase3 | kept |
|---|---|---|---|---|---|---|
| F2c | 0.921 | 26.5 | 1.5 | 2.5 | 23 | 91 |
| `gz=35` (default baru) | 0.894 | 19.6 | 0.5 | 3.1 | 21 | 92 |

Foul masih ~2× EPL. Percobaan tekel tetap 182 per laga (first defender menekan, intent PRESS_BALL 97%).

## Berikutnya

1. Offside (11.5 per tim vs 1.8).
2. Foul sisa: frekuensi tekel first defender.
3. Tembakan dan corner.

## Hasil tonton manual (seed 19, default `7f2ec35`) + probe offside

Probe `AUDIT offside standing` / `AUDIT offside passes` (read-only, output identik).

1. **Offside kebanyakan "timing", bukan posisi.** Waktu penyerang berdiri offside di TS justru lebih sedikit dari jalur lama (0.09–0.12 vs 0.12–0.15 per langkah pegang bola). Tapi 24 dari 31 umpan offside (seed 19) adalah undian timing: penerima masih onside 0–3 m, peluang dinyatakan offside sampai 70% (`Passing_OFFSIDE_TIMING_CHANCE 0.7`, jendela 3 m). TS memberi lebih banyak umpan maju ke pelari yang sejajar garis, jadi undian itu lebih sering kepakai. Sisanya (~7) umpan ke penerima 0–2 m di depan garis: M2 membaca garis dengan toleransi 1 m.
2. **Kawalan lengket.** Di layar, bek menempel 1–2 m di penyerang (Kerkez–Doku, Gvardiol/Dias–Isak); di Metrica jaraknya ~7 m. Scorecard: bek <3 m dari penyerang 2.7 vs 1.5, jarak penyerang→bek 5.3 vs 7.4. Ini juga memberi banyak kontak/tekel.
3. **Garis belakang terlalu tinggi saat bertahan di tengah:** 41 m vs 32 m. Bola di belakang garis jadi sering.
4. **Pergantian karena cedera di babak 1:** 1.2 per laga (go16 0.8). Sudah ada sebelum TS.

## Ronde 2: kawalan lengket

**Ukur akar** (`viz/markprof.js`, bertahan settled): jarak bek terdekat ke tiap penyerang, per jarak penyerang ke bola.

| Jarak ke bola | Metrica med / % <3 m | F5 slide gate | go16 |
|---|---|---|---|
| 0–8 m | 4.7 / 29% | 3.6 / 41% | 4.8 / 29% |
| 8–15 m | 5.2 / 23% | 4.2 / 32% | 5.9 / 19% |
| 15–25 m | 6.0 / 18% | 4.3 / 32% | 6.8 / 14% |
| 25–35 m | 6.4 / 18% | 4.9 / 27% | 7.7 / 12% |

Di bola asli jarak kawal melebar makin jauh dari bola. Marker native selalu 2 m goal-side untuk semua penyerang dalam 25 m (40 m saat press).

**Perubahan:** `TeamShape.MarkSlope` (kunci `ml`, default 0.15). Di luar zona bahaya, jarak marker = 2 m + 0.15 × jarak penyerang ke bola. Zona bahaya tetap 2 m.

| 10 seed | Skor | Lolos | 15–25 m % <3 m | 25–35 m % <3 m | Gol | Foul/tim | Offside/tim |
|---|---|---|---|---|---|---|---|
| slide gate | 0.894 | 94/138 | 32% | 27% | 3.1 | 19.6 | 11.5 |
| `ml=0.15` (default baru) | **0.816** | 96/138 | 22% | 17% | 2.3 | 21.1 | 9.2 |

Penjaga bentuk tidak memburuk (own3in5 36, tepi bawah). Gol turun 3.1 → 2.3: diawasi, masih dalam noise 10 seed (~0.55 SD).

**Sisa:** dekat bola (0–15 m) masih terlalu rapat (40% vs 29%): first defender, cover, dan presser kedua, bukan marker.

## Ronde 3: rapat dekat bola

Probe `AUDIT near-ball marking` (read-only): dari penyerang dalam 15 m dari bola, ~28% punya bek <3 m. Penyumbang: marker ~13%, cover ~7%, first defender ~4.5%, presser kedua ~2.4%.

| 10 seed | Skor | Lolos | Penjaga bentuk | 0–8 m % <3 m | Sepertiga sendiri bek <3 m (asli 2.4) | Gol | Offside/tim |
|---|---|---|---|---|---|---|---|
| default `ml=0.15` | 0.816 | 96 | 2/6 | 40% | 3.3 | 2.3 | 9.2 |
| `mo=4,ml=0.08` (jarak dasar marker) | 0.858 | 96 | 2/6 | 39% | 3.3 | 3.2 | 9.1 |
| `mo=3.5,ml=0.1` | 0.800 | 98 | 2/6 | 40% | 3.4 | 2.4 | 8.1 |
| `mt=18` (ketat hanya ±kotak) | 0.809 | 98 | **1/6** (own3in5 35) | 47% | 2.6 | 2.3 | 8.5 |
| **`mt=25` (default baru)** | **0.792** | 99 | 2/6 | 40% | 3.0 | 3.5 | 7.0 |

- **Jarak dasar marker (`mo`):** gagal, profil dekat bola tidak bergerak. Dibuang.
- **Temuan:** di sepertiga sendiri seluruh zona bahaya (35 m dari gawang) dikawal 2 m.
- **Perubahan:** `TeamShape.MarkTight` (kunci `mt`, default 25). Di zona bahaya hanya penyerang dalam 25 m dari gawang yang dikawal 2 m; yang lebih jauh memakai jarak melebar `MarkSlope`.

**Sisa:** 0–8 m dari bola tetap 40% (asli 29%). Itu bukan marker, tapi cover/first defender/presser kedua di sekitar pembawa bola.

## Ronde 4: bek di sekitar pembawa bola (stop-loss)

**Ukur** (`viz/crowd.js`, bertahan settled): jumlah bek di sekitar pembawa bola sudah realistis.

| | Metrica | Default `mt=25` |
|---|---|---|
| Bek dalam 5 / 10 / 15 m dari pembawa | 0.60 / 1.64 / 2.97 | 0.68 / 1.71 / 2.92 |
| Bek terdekat / kedua / ketiga (median) | 5.2 / 10.2 / 13.9 m | 4.2 / 9.9 / 14.1 m |
| Teman pembawa dalam 10 m | 0.55 | 0.68 |

**Hipotesis:** first defender contain terlalu dekat (2 m). Knob uji `cz`: jarak contain di luar press dan di luar zona bahaya.

| 10 seed | Skor | Lolos | Bek terdekat | 0–8 m % <3 m | carry25 | Foul/tim |
|---|---|---|---|---|---|---|
| default | 0.792 | 99 | 4.2 m | 40% | 18.8 | 21.0 |
| `cz=3.5` | 0.836 | 93 | 4.2 m | 39% | 28 | 20.1 |
| `cz=4.5` | 0.879 | 89 | 4.6 m | 39% | 39 | 20.0 |

**Gagal:** jarak ke pembawa hampir tidak berubah, zona 0–8 m tidak bergerak, dan pembawa mendapat ruang untuk giring panjang. Knob dibuang.

**Stop-loss untuk zona 0–8 m.** Tiga hipotesis (jarak dasar marker, ketat-dekat-gawang, jarak contain) tidak menggerakkan angka 40% vs 29%. Diterima sebagai gap terdokumentasi. Petunjuk untuk nanti: angka ini lebih dipengaruhi teman pembawa yang terlalu dekat (0.68 vs 0.55 dalam 10 m, sisi menyerang) daripada posisi bek.

## Ronde 5: offside

**Ukur** (default `mt=25`, 10 seed, per laga dua tim): 15.4 umpan offside = 11.2 undian timing (penerima masih onside 0–3 m dari garis) + 4.2 benar-benar lewat garis. Asli ~3.6 per laga.

**Perubahan:** `M2.OffsideTiming` (kunci `ot` di `FMENGINE_M2TUNE`), peluang undian timing Player12 0.7 → **0**.

| 10 seed | Offside/tim | Gol | Tembakan/tim | Skor | Penjaga bentuk |
|---|---|---|---|---|---|
| `ot=0.7` (lama) | 7.0 | 3.5 | 21.8 | 0.792 | 2/6 |
| `ot=0.2` | 4.8 | 5.0 | 21.6 | 0.811 | 2/6 |
| **`ot=0` (default baru)** | **2.4** | 3.9 | 22.5 | 0.832 | 2/6 |

Offside mendekati EPL (1.8). Gol 3.9 masih di batas +20%. Skor naik 0.04 hanya karena metrik transisi dan penerimaan bola: umpan terobosan yang dulu dimatikan undian sekarang sampai. Bentuk fase settled tidak berubah.

**Catatan reproduksi:** M6.SlideDepth dan M2.OffsideTiming juga berlaku di `FMENGINE_OFF=TS`. go16 persis hanya dari tag `pre-ts-default`.

**Berikutnya:** tembakan (22.5 per tim vs ~13) dan corner (12 vs ~5.3) sekarang outlier statistik terbesar, lalu foul (20 vs 10.5).

## Ronde 6: tembakan

**Bug pengukuran (`b133c2a`):** event `SHOT_OFF_TARGET` tidak pernah dicatat. Tembakan melenceng keluar tidak masuk statistik laga maupun replay (seed 19: 46 tercatat dari 71). Semua angka tembakan sebelumnya ~40% terlalu kecil. Sekarang dicatat; frame identik.

**Probe `AUDIT shots`** (default, 10 seed, per laga dua tim): 71 tembakan; 28 dari >25 m; 35 dengan xG <0.03; 51% diblok; 26 dari bola mati (13 sundulan); xG total 4.7.

**Perubahan:** `M2.ShotValueScale` 1.7 → **1.2** (nilai lama dikalibrasi dengan hitungan tembakan yang kurang).

| 10 seed | Tembakan/tim | >25 m per laga | Diblok | Gol | Skor | Penjaga bentuk |
|---|---|---|---|---|---|---|
| `ss=1.7` | 33.2 | 27.9 | 51% | 3.9 | 0.832 | 2/6 |
| **`ss=1.2` (default)** | 30.6 | 17.7 | 56% | 3.9 | 0.792 | 2/6 |
| `ss=0.9` | 24.8 | 13.3 | 55% | 4.3 | 0.783 | 1/6 (recvPress 38) |

`ss=0.9` ditolak: penjaga recvPress jebol, gol di atas +20%.

**Sisa (masih ~2× EPL 13/tim):**
1. Porsi diblok ~55% (asli ~25%): blok terlalu mudah, atau tembakan dilepas ke tubuh bek.
2. Tembakan dari bola mati ~26 per laga, termasuk ~13 sundulan (EPL ~5 sundulan per laga); corner ~12 per tim (asli 5.3).

## Ronde 7: blok tembakan

**Ukur:** jalur tembakan (open play) saat bola ditendang, bek outfield dalam jarak dari garis penembak → tengah gawang.

| | Metrica (48 tembakan) | Engine (6 seed) |
|---|---|---|
| Bek dalam 1.6 m: 0 / 1 / 2+ | 27% / 52% / 21% | 25% / 61% / 13% |
| Bek dalam 1.0 m: 0 / 1 / 2+ | 48% / 46% / 6% | 41% / 52% / 7% |
| Diblok | 6% (Opta EPL ~25%) | 52–56% |

Pemilihan tembakan sudah realistis. Yang terlalu mudah adalah fisika blok: bek sampai 1.6 m dari jalur bola memblok dengan peluang 35–80%.

**Perubahan:** `M6.BlockReach` (kunci `br`), 1.6 m (Player12) → **0.7 m**.

| 10 seed | Diblok | Tembakan/tim | Tembakan susulan <10 dtk | Corner/tim | Gol | Tepat sasaran/tim | Skor |
|---|---|---|---|---|---|---|---|
| `br=1.6` | 56% | 30.6 | 8.6 | 12.3 | 3.9 | 5.0 | 0.792 |
| `br=1.0` | 35% | 24.3 | 3.7 | 8.9 | 4.2 | 6.0 | 0.773 |
| **`br=0.7` (default)** | **25%** | 23.1 | 2.9 | 7.8 | 4.6 | 6.4 | **0.763** |

Penjaga bentuk sama (2/6). Scramble setelah blok berkurang, corner turun.

**Konsekuensi:** gol 4.6 per laga, di atas batas +20% (3.94); tepat sasaran 6.4 per tim (EPL ~4.5). Berikutnya: kualitas/xG tembakan dan kiper. Tembakan masih ~23 per tim (EPL ~13).

## Ronde 8: gol (volume vs konversi)

**Ukur** (default `br=0.7`, 10 seed, per laga): 51 tembakan (EPL ~26), tepat sasaran 12.8 (~9), gol 4.6 (3.28). Save% kiper 65% (asli ~64–67%), gol per tembakan 0.090 (asli ~0.126).

`viz/attacks.js` (penguasaan bola per laga):

| | Metrica | Engine |
|---|---|---|
| Penguasaan yang masuk sepertiga akhir | 138.5 | 136.8 |
| Penguasaan yang masuk kotak (bola dikuasai) | 32.0 | 33.6 |
| Tembakan | 24 | 51 |

Serangan sudah realistis; engine menembak 2× lebih sering per serangan.

**Perubahan:** `M2.ShotValueScale` 1.2 → **0.8**.

| 10 seed | Tembakan/tim | Tepat sasaran/tim | xG/laga | Gol | Skor | Penjaga bentuk |
|---|---|---|---|---|---|---|
| `ss=1.2` | 23.1 | 6.4 | 4.25 | 4.6 | 0.763 | 2/6 |
| **`ss=0.8` (default)** | 18.3 | 5.7 | 3.97 | 4.6 | 0.779 | 2/6 |
| `ss=0.6` | 14.9 | 4.5 | 3.33 | 4.5 | 0.764 | 1/6 (recvPress 38) |

`ss=0.6` ditolak: recvPress jebol (38) untuk kedua kalinya di nilai `ss` rendah.

**Temuan utama:** gol tidak bergerak saat tembakan dipangkas 35%. Di `ss=0.6` xG 3.33 tetapi gol 4.5: engine mengonversi ~35% di atas xG-nya sendiri. Akar gol ada di fisika penyelesaian/kiper (konversi aktual vs `estimateShotXG`), bukan volume tembakan. Langkah berikutnya: bandingkan gol aktual per bin xG.

## Ronde 9: konversi dan penalti

**Koreksi ronde 8:** probe gol per bin xG (10 seed, default `ss=0.8`) menunjukkan gol hanya ~11% di atas xG engine (44 gol dari tembakan vs xG 39.7). Angka "35%" berasal dari varian `ss=0.6`, bukan default.

| Bin | Tembakan (10 laga) | Konversi | Gol / xG |
|---|---|---|---|
| Penalti | 12 | 92% | 1.18 |
| Open play xG 0.05–0.10 | 78 | 10.3% | 1.41 |
| Open play xG 0.10–0.30 | 60 | 18.3% | 1.09 |
| Sundulan xG 0.10–0.30 | 61 | 14.8% | 1.20 |
| Open play xG <0.03 | 132 | 0.8% | 0.45 |

**Akar sebenarnya: penalti 1.2 per laga** (EPL ~0.26), menyumbang ~1.1 gol per laga. Probe foul: 1.4 foul per laga di kotak sendiri, semuanya tekel di darat (duel udara 0). Porsi foul di kotak 3% (asli ~1.4%).

**Perubahan:** `M6.BoxFoulScale` (kunci `bx`), faktor foul tekel di kotak sendiri 0.13 (Player12) → **0.03**.

| 10 seed | Penalti/laga | Gol/laga | xG/laga | Skor | Lolos | Penjaga bentuk |
|---|---|---|---|---|---|---|
| `bx=0.13` | 1.20 | 4.6 | 3.97 | 0.779 | 101 | 2/6 |
| `bx=0.06` | 0.50 | 3.5 | 3.52 | 0.770 | 102 | 2/6 |
| **`bx=0.03` (default)** | 0.40 | **3.4** | 3.45 | **0.768** | 102 | 2/6 |

Gol sekarang di dalam ±20% EPL (3.28).

**Sisa statistik:** tembakan ~19 per tim (EPL ~13), foul ~23 per tim (10.5), kartu merah 1.3 per laga (0.15), corner ~8 per tim (5.3).

## Ronde 10: foul dan kartu

**Ukur** (default `bx=0.03`, 10 seed, per laga): 215 percobaan tekel (EPL kira-kira 40–50), 46 foul.

| | Sliding | Standing |
|---|---|---|
| Percobaan | 112 | 103 |
| Meleset | 60 (53%) | 12 (11%) |
| Menang | 24 | 45 |
| Foul | ~28 (mayoritas reckless) | ~6 |

Merah 1.3 per laga = 0.5 merah langsung + 0.8 kuning kedua (kuning 7.9 per laga, EPL ~4.2).

**Hipotesis:** sliding masih terlalu sering; `SlideDepth` 35 m masih mencakup sebagian besar fase bertahan.

| | Foul/tim | Kuning | Merah | Sliding/laga | Gol | carry25 | Skor | Penjaga |
|---|---|---|---|---|---|---|---|---|
| `gz=35` (10 seed) | 22.9 | 7.9 | 1.3 | 112 | 3.4 | 22 | 0.768 | 2/6 |
| `gz=20` (10 seed) | 19.7 | 5.0 | 0.7 | 83 | 4.5 | 22 | 0.745 | 3/6 |
| `gz=1` (10 seed) | 8.3 | 1.9 | 0.0 | 35 | 3.6 | 32 | 0.804 | 2/6 |

`gz=1` ditolak: tanpa sliding bek tidak bisa menghentikan pembawa bola (carry25 32, skor memburuk).

**Cek 20 seed** (tambahan seed 1 2 4 9 10 12 14 15 16 17):

| 20 seed | Gol | Foul/tim | Kuning | Merah | Skor |
|---|---|---|---|---|---|
| `gz=35` | 3.8 | 22.7 | 8.6 | 1.1 | 0.764 |
| **`gz=20` (default baru)** | 4.3 | 19.7 | 5.6 | 0.75 | 0.769 |

Disiplin membaik jelas, bentuk netral. Selisih gol +0.5 sekitar 0.8 SD di 20 seed; dipantau.

**Pelajaran noise:** antara dua set 10 seed, skor bentuk bergeser ~0.03 dan gol ~0.6 untuk config yang sama. Perbaikan skor di bawah ~0.03 dari ronde sebelumnya belum tentu nyata. Keputusan statistik gol sebaiknya pakai 20 seed.

**Sisa akar foul:** percobaan tekel ~196 per laga, ~4× EPL. First defender (intent PRESS_BALL) menekel begitu bola masuk jangkauan.

## Ronde 11: frekuensi tekel (gagal, dibuang)

**Hipotesis:** first defender commit begitu bola masuk jangkauan; knob uji `tr` = laju commit per detik saat bola masih di kaki pembawa (jockey), kecuali sentuhan lepas atau standoff.

| 10 seed | Percobaan tekel | Foul/tim | Merah | Skor | Penjaga |
|---|---|---|---|---|---|
| default `gz=20` | 196 | 19.7 | 0.7 | 0.745 | 3/6 |
| `tr=1.5` | 133 | 16.6 | 1.7 | 0.768 | 1/6 (recvPress 38) |
| `tr=0.7` | 126 | 16.3 | 1.2 | 0.817 | 1/6 (recvPress 38) |

**Gagal:** tekel standing turun (~103 → ~45), tetapi foul hanya turun ~3 per tim. Sliding tetap ~85 per laga dan foul sliding meleset-reckless (~20 per laga) adalah sumber foul dan merah langsung. Penjaga recvPress jebol. Knob dibuang.

**Asal sliding** (probe `slide recover/forced`, 5 seed, per laga):
- ~59: bek mengejar dari belakang dalam 20 m dari gawang (lewat `SlideDepth`);
- ~28: take-on yang gagal (`feTakeOn`): bek yang "membaca" tetapi di luar jangkauan standing langsung sliding, **di mana saja** (tidak lewat `SlideDepth`);
- 0: standoff paksa.

**Berikutnya:** take-on slide di luar zona sliding → jockey (tanpa commit); dan syarat sliding pemulihan dekat gawang (mis. hanya saat pembawa lari ke gawang).

## Ronde 12: sliding take-on (gagal) dan stop-loss foul

**Hipotesis:** sliding dari take-on yang gagal tidak lewat `SlideDepth`; knob uji `tg` = di luar zona sliding bek menahan (tidak commit).

| 20 seed | Foul/tim | Kuning | Merah | Sliding/laga | Gol | carry25 | Skor | recvPress (≤37) |
|---|---|---|---|---|---|---|---|---|
| default | 19.7 | 5.6 | 0.75 | 84 | 4.3 | 22.5 | 0.769 | 37 / 36 |
| `tg=1` | 17.2 | 6.3 | 0.85 | 60 | 4.4 | 25.5 | 0.768 | 37 / 38 |

**Ditolak:** foul −13% tetapi kartu tidak membaik, giring panjang naik, recvPress jebol di satu set. Sliding pemulihan meleset-reckless dekat gawang (~19 per laga), sumber utama kartu, tidak tersentuh.

**Stop-loss foul** (dua kegagalan berturut-turut: `tr`, `tg`). Diterima sebagai gap terdokumentasi:
- foul ~20 per tim (EPL 10.5);
- kuning ~5.6, merah ~0.75 per laga (EPL 4.2 / 0.15);
- percobaan tekel ~196 per laga.

Petunjuk untuk nanti: sliding pemulihan dalam 20 m dari gawang (~59 per laga, meleset ~50%) dan reckless = sliding dari belakang atau kecepatan ≥5 m/s (`feRecklessSpeed`).

## Ronde 13: chase — marker yang kelewat bola (flag, default off)

**Ukur** (probe `AUDIT chase>=3`, seed 19): pelari di momen chase≥3: marker 23% + marker yang sudah kelewat bola ~25% (lari balik 5–7 m/s), slot 15%, first defender/cover/screen 16%. Aturan "yang kelewat bola jogging balik" (`TrotSpeed` 2.6) hanya berlaku untuk pemegang slot; marker mengejar orangnya dengan kecepatan orang itu + menutup jarak, hingga `RunSpeed` 5.5.

**Perubahan uji:** `TeamShape.BypassTrot` (kunci `bt`), marker yang kelewat bola juga jogging balik. Helper `teamShapeBypassed` dipakai bersama.

| 20 seed (utama / tambahan) | chase3 (5–10) | liar5 (6–10.5) | recvPress (≤37) | Gol | Skor |
|---|---|---|---|---|---|
| default | 17.2 / 17.7 | 24 / 24 | 37 / 36 | 4.5 / 4.1 | 0.745 / 0.793 |
| `bt=1` | 14.7 / 14.3 | 19.9 / 19.9 | 39 / 38 | 4.4 / 5.4 | 0.781 / 0.780 |

Target bergerak konsisten, tetapi penjaga recvPress jebol di dua set dan gol naik ~0.6. **Tidak dijadikan default**; flag disimpan (off).

**Pola:** recvPress duduk di tepi atas rentang (36–37). Hampir setiap perubahan yang melonggarkan pertahanan (`ss` rendah, `tr`, `tg`, `bt`) mendorongnya ke 38–39 dan menaikkan gol. Sebelum melonggarkan pertahanan lagi, recvPress perlu dibedah sendiri (siapa yang menekan penerima, dan apakah definisinya sebanding dengan Metrica).

## Ronde 14: bedah recvPress

`press.js`: penerimaan bola = pergantian pemegang dalam satu tim (jeda ≤4 dtk); "ditekan" = lawan outfield terdekat <4 m saat menerima. Penjaga lama memakai angka gabungan, rentang 24–37 (Metrica..SkillCorner).

`audit/recvzone.js` memecah per zona penerima (dari arah serang timnya):

| Zona | Ditekan: Metrica / SkillCorner | Ditekan: default / `bt=1` | Dipertahankan: Metrica / SkillCorner | Dipertahankan: default |
|---|---|---|---|---|
| Sepertiga sendiri | 23% / 16% | **11%** / 13% | 74% / 87% | **98%** |
| Tengah | 22% / 32% | **37%** / 37% | 83% / 86% | **93%** |
| Sepertiga akhir | 38% / 57% | **63%** / 64% | 62% / 69% | **82%** |
| Porsi penerimaan (sendiri/tengah/akhir) | 20/56/24 / 29/51/20 | 28/47/26 / 26/46/29 | | |
| Gabungan | 26% / 32% | 37% / 38–39% | | |

**Temuan:**
1. Angka gabungan menyembunyikan dua error berlawanan: build-up lawan terlalu jarang ditekan (11% vs 16–23%), tengah dan sepertiga akhir terlalu sering (37% vs 22–32%, 63% vs 38–57%). Default lolos penjaga lama secara kebetulan; per zona ketiganya di luar rentang.
2. `bt=1` tidak memperburuk zona mana pun (sepertiga sendiri sedikit membaik). Angka gabungan naik karena porsi penerimaan di sepertiga akhir bertambah (26% → 29%), bukan karena tekanan per zona.
3. Masalah nyata: bola dipertahankan terlalu mudah di semua zona (penjaga `kept` 91, rentang 74–86), dan build-up lawan hampir tidak ditekan.

**Usulan (menunggu keputusan):** ganti penjaga recvPress gabungan dengan tiga penjaga per zona (rentang Metrica..SkillCorner) supaya tidak menghukum pergeseran komposisi.

## Ronde 15: penjaga per zona + BypassTrot default

**Penjaga diganti** (keputusan user, 2026-11-23): `recvPress` gabungan (24–37) → tiga penjaga bentuk per zona penerima (`audit/recvzone.js`, rentang Metrica..SkillCorner dilebarkan sedikit):

| Penjaga | Rentang |
|---|---|
| `recvOwn` | 15–25 |
| `recvMid` | 21–33 |
| `recvFinal` | 36–58 |

Dengan penjaga baru default lama gagal di ketiganya (11 / 37 / 63); sebelumnya lolos karena error berlawanan saling menutupi.

**`TeamShape.BypassTrot` jadi default** (20 seed, ronde 13):

| 20 seed | chase3 | liar5 | recvOwn / Mid / Final | Gol | Foul/tim | Skor |
|---|---|---|---|---|---|---|
| sebelum | 17.5 | 24 | 11.5 / 36.5 / 62.5 | 4.3 | 19.7 | 0.769 |
| **`bt=1`** | **14.5** | **19.9** | 13.5 / 36.5 / 62.5 | 4.9 | 18.8 | 0.781 |

Tidak ada penjaga per zona yang memburuk; recvOwn membaik. Gol +0.6 diterima, untuk dikejar lewat penjaga `kept`.

**Berikutnya:**
1. **Bola terlalu mudah dipertahankan:** kept per zona 98 / 93 / 82 vs asli 74–87 / 83–86 / 62–69, bahkan saat ditekan. Kemungkinan juga menekan gol turun.
2. Build-up lawan kurang ditekan (recvOwn 13 vs 15–25).

## Ronde 16: kept (bola terlalu mudah dipertahankan)

**Ukur** (`audit/turnover.js`, pergantian pemegang dalam live play):

| Per laga | Metrica (event) | Metrica (replay) | SkillCorner | Engine (default) |
|---|---|---|---|---|
| Bola hilang | ~230 | 243 | – | 106 |
| Porsi pergantian dari semua perpindahan bola | ~27% | 25% | 20% | **7.9%** |

**Kalibrasi umpan** (`FMENGINE_M2TRACE=1`, `PASSOUT`, 3 seed): 1330 umpan per laga, sukses 94.8% (EPL ~82%). Di setiap tingkat keyakinan M2 hasilnya lebih sukses dari keyakinan: 0.5–0.6 → 82%, 0.3–0.4 → 61%. 71% umpan diyakini ≥0.9.

**Uji `ib`** (`M6.InterceptBase`, peluang dasar sentuhan potong 0.62 → 0.8 / 0.95, 20 seed): porsi bola hilang 7.6% → 8.5–8.8%, kept 91 → 90–91. **Gagal**, dibuang. Bek jarang berada dalam jangkauan bola; peluang sentuhan bukan pembatas.

**Bentuk umpan sudah mirip asli:**

| | Metrica | Engine |
|---|---|---|
| Umpan per laga | ~860 | 1331 |
| Jarak median | 13.1–13.9 m | 12.0 m |
| Maju >10 m | 19–21% | 15% |
| Mundur | 38–39% | 38% |
| Sukses | 76–81% | 95% |

Dugaan "terlalu main aman" melemah: geometri umpan sama, tetapi di bola asli ~1 dari 5 hilang, di engine 1 dari 20.

**Tersangka berikutnya:**
1. Bek tidak pernah berdiri di jalur umpan (ukur okupansi jalur umpan saat ditendang, Metrica vs engine, seperti jalur tembakan).
2. Umpan dan kontrol pertama terlalu sempurna (Metrica: ~40 BALL LOST tanpa potong + ~38 BALL OUT per laga).

## Ronde 17: jalur umpan — bek atau pengumpan?

**Ukur** (`audit/passlane.js`, `audit/lanecover.js`):

| | Metrica | SkillCorner | Engine |
|---|---|---|---|
| Umpan yang ditendang lewat jalur dengan bek ≤1 m | 9% | 10% | **2%** |
| ≤2 m | 20% | 21% | 15% |
| Umpan hilang | 16.5% | 12.3% | 4.1% |
| **Semua opsi umpan** (teman 5–30 m) yang jalurnya ditutup bek ≤1 m | 19% | – | 23% |
| ≤2 m | 37% | – | 40% |

**Bek engine memotong jalur sama seringnya dengan bek asli.** Yang berbeda adalah pengumpannya: pemain asli tetap mengumpan lewat jalur yang tertutup (9–10%) dan sebagian kehilangan bola; pengumpan engine hampir tidak pernah (2%). Model M2 menganggap bek dalam 1.25 m dari jalur pasti memotong (fisika hanya ~50–60%), jadi pengumpan terlalu takut dan hanya memilih umpan yang sempurna aman.

**Uji persepsi pengumpan** `M2.InterceptRadius` (`ir`, 1.25):

| 20 seed (utama / tambahan) | Bola hilang | kept (74–86) | possMed (4.5–7.5) | bwMid (22–34) | carry25 | Tembakan/tim | Gol | Kuning | Skor | Penjaga bentuk / keputusan |
|---|---|---|---|---|---|---|---|---|---|---|
| default `ir=1.25` | 7.6% | 91 / 91 | 12.5 / 12.9 | 32 / 33 | 21 / 22 | 21.2 | 4.4 / 5.4 | 5.7 | 0.775 | 1 / 3 dari 8 / 5 |
| `ir=0.8` | 11.9% | 88 / 89 | 9.2 / 9.4 | 26 / 27 | 16 / 13 | 18.5 | 5.4 / 4.3 | 4.1 | 0.805 / 0.832 | 2–3 / 3 |
| `ir=0.5` | 16% | **85 / 86** | **6.8 / 6.8** | **23 / 24** | 10 / 11 | 16.5 | 4.7 / 4.4 | 3.9 | 0.842 | 2 / **5** |

`ir=0.5` meloloskan **semua** penjaga keputusan untuk pertama kali (kept, possMed, bwMid), plus recvOwn (19), kartu kuning dan tembakan mendekati EPL. Biayanya skor bentuk +0.067 (di atas noise 0.03):
- giring ≥25 m per 90 (scorecard) 12 → 5.2: pemain mengumpan alih-alih menggiring;
- penerimaan bergeser dari FB/CB ke AM, lebar dan jalur luar saat build-up menyempit;
- recvMid 37 → 46, recvFinal 62 → 70;
- metrik transisi membaik.

`ir=0.8` tidak meloloskan kept/possMed. **Belum dijadikan default**; menunggu keputusan (nilai tengah ~0.65, atau terima 0.5).

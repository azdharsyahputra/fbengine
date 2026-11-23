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

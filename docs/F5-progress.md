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

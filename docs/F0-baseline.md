# F0: patokan dan baseline bentuk tim (2026-11-23)

Bagian dari [PLAN-team-shape.md](PLAN-team-shape.md). Belum ada perubahan engine.

## Alat

- `fm-game/viewer/cek/metrica_to_replay.py` (fm-game, bukan repo ini):
  - Konversi Metrica sample game 1–2 (laga penuh, 25 Hz, 22 pemain selalu terlihat) ke replay schema 2.
  - Live mask dari event: bola keluar, tembakan keluar/gol, foul, kartu, sampai restart berikutnya. Goal kick termasuk restart. Hasilnya live 49% dan 58%.
  - Peran ditebak dari rata-rata posisi satu laga (urutan kedalaman lalu lebar). Pergantian pemain dipetakan ke slot.
- `audit/shape_suite.js`: metrik S1–S10 + kecepatan per fase (tenang/transisi) dan zona bola. Sama untuk engine, Metrica, dan SkillCorner.
- `audit/formation_table.js`: median posisi per slot per fase dan per sel bola (3×3), di frame serang tim.
  - Kiri/kanan dinormalisasi per tim per babak, karena engine cuma mencerminkan sumbu x.
  - Tabel Metrica: `refdata/derived/formation_metrica.json` (tidak di-commit).

**Validasi silang:** distribusi penerima bola di tengah, Metrica vs SkillCorner (EPL/LaLiga):

| | CB | FB | DM | AM | ST |
|---|---|---|---|---|---|
| Metrica | 16% | 24% | 28% | 29% | 4% |
| SkillCorner | 15–22% | 16–23% | 27–31% | 22–31% | 3–6% |

Cocok, jadi Metrica layak jadi patokan.

## Baseline (fase tenang kecuali disebut; go16 = 10 seed, P12 murni = 5, r53 = 3, Metrica = 2 laga)

| Metrik | Metrica | go16 | P12 | r53 |
|---|---|---|---|---|
| Lebar pegang bola: sepertiga sendiri | 44.8 | 31.5 | 33.5 | 40.1 |
| Lebar pegang bola: tengah | 45.6 | 36.2 | 34.2 | 44.4 |
| Lebar pegang bola: akhir | 36.8 | 38.9 | 33.3 | 43.8 |
| Panjang pegang bola: tengah | 29.7 | 25.8 | 37.6 | 34.9 |
| Pasangan setim <4 m: sendiri | 0.73 | 1.01 | 0.62 | 0.49 |
| Pasangan setim <4 m: tengah | 0.09 | 1.00 | 0.60 | 0.26 |
| Jarak teman terdekat: tengah | 12.1 | 8.2 | 10.6 | 12.0 |
| % kedua jalur pinggir terisi: sendiri | 59 | 22 | 17 | 49 |
| % kedua jalur pinggir terisi: tengah | 72 | 28 | 10 | 77 |
| Jarak bek–DM, pegang bola, tengah | 9.6 | **1.3** | 2.9 | 4.5 |
| Jarak DM–AM, pegang bola, tengah | 9.2 | 11.4 | 21.5 | 18.2 |
| Lebar blok bertahan: tengah | 33.5 | 29.0 | 30.7 | 30.5 |
| Panjang blok bertahan: sendiri | 26.9 | 30.9 | 21.0 | 22.2 |
| Panjang blok bertahan: tengah | 24.1 | **35.2** | 26.7 | 23.6 |
| Panjang blok bertahan: akhir | 29.3 | 37.0 | 33.3 | 32.6 |
| Panjang blok bertahan, transisi: tengah | 26.0 | 37.8 | 36.1 | 29.0 |
| Jarak DM–AM bertahan: tengah | 6.7 | 12.9 | 12.7 | 9.6 |
| Bek <3 m dari penyerang: akhir | 1.59 | 1.07 | 1.33 | 2.06 |
| Penerima tengah CB/FB/DM/AM/ST (%) | 16/24/28/29/4 | 6/17/15/**48**/14 | 20/17/14/39/10 | 10/30/31/24/5 |
| Penerima sendiri CB/FB/DM/AM/ST (%) | 40/22/18/11/0 | 30/32/23/14/1 | 41/38/9/8/0 | 53/18/14/15/0 |
| Kecepatan <2 / 2–4 / 4–5.5 / 5.5–7 / >7 m/s (% waktu) | 48/39/10/3/1 | 30/40/18/9/2 | 24/38/30/6/2 | 46/36/8/8/2 |

**Tabel formasi** (median u/w, bola di tengah dan tengah-lebar, tim pegang bola). u = meter dari garis gawang sendiri, w = meter dari sisi kiri.

| Slot | Metrica | go16 |
|---|---|---|
| FB-L | 50/17 | 46/16 |
| CB-L | 40/30 | 40/28 |
| CB-R | 43/46 | 40/41 |
| FB-R | 51/58 | 47/51 |
| DM-L | 59/25 | 44/31 |
| DM-R | 53/34 | 46/40 |
| AM-L | 66/20 | 57/20 |
| AM-C | 66/39 | 54/35 |
| AM-R | 61/43 | 56/48 |
| ST | 70/31 | 62/34 |

Artinya:
- Pas pegang bola di tengah, **pivot go16 turun ke garis bek** (DM 44–46 vs 53–59), jadi ada "6 bek".
- AM dan ST 8–12 m lebih dalam dari Metrica.
- Pas bertahan di tengah, garis bek go16 4–5 m lebih dalam (29 vs 33–35), sementara AM/ST 10–12 m lebih tinggi (54/67 vs 44–53/55). Itu sumber blok yang molor.

## Audit ulang target go13–go16

| Putaran | Target dulu (SkillCorner) | Metrica | go16 | Status |
|---|---|---|---|---|
| go13 | giring ≥25 m: 0–2 per laga | **11–14** (≥15 m: 45–60) | 4–7 (≥15 m: 24–30) | **Kebablasan.** Sekarang malah kurang. |
| go14 | CB <2 m: 0–1% | 0–0.7% | 0.7–3.6% | Oke |
| go14 | dua FB di setengah sama: 2–6% | **12–27%** | 1–12% | **Kebablasan** (`m1.full_back_cross`) |
| go14 | jarak CB tengah/akhir | 11.4–11.9 / 13.7–18.2 m | 9.2–9.3 / 9.6–10.6 m | Terlalu rapat makin ke depan |
| go15 | FB non-pemegang vs bola, tengah / akhir | −9..+3 / −20..−9 m | −11..−9 / −12..−11 m | Oke |
| go15 | FB pegang bola lebar di sepertiga akhir | 11–29% | 23–25% | Oke |
| go16 | penerima tengah | lihat tabel | AM 48 vs 29 | Masih gap (akibat struktur) |

**Metrik cek lama yang beda antara Metrica dan SkillCorner.** Ini karena kamera atau gaya main, jadi dipakai sebagai **rentang**, bukan titik:

| Metrik | Metrica | SkillCorner | go16 |
|---|---|---|---|
| chase≥3 | 9.0 | 5.8 | 5.7 |
| liar5 | 9.0 | 6.8 | 12.3 |
| Umpan mundur | 33% | 24% | 23% |
| possMed | 4.7 dtk | 6.3 dtk | 10.4 dtk |
| Umpan mundur sepertiga akhir | 47% | 31% | 37% |
| Umpan mundur tengah | 32% | 25% | 21% |

- Ping-pong Metrica 51 itu artefak deteksi pemegang di duel. Tidak dipakai.
- Tekanan build-up (own3in5): Metrica 44, SkillCorner 40, go16 28. Kurang di go16.
- mid3in5: Metrica 46, SkillCorner 51, go16 46. Oke.

## Gap terverifikasi (urut besar → kecil)

1. **Struktur pegang bola di sepertiga sendiri dan tengah:**
   - lebar 31–36 vs 45 m;
   - jalur pinggir cuma terisi 22–28% (asli 59–72%);
   - pivot nempel garis bek (bek–DM 1.3 vs 9.6 m);
   - tumpukan di tengah 1.0 vs 0.09 pasangan per frame;
   - AM/ST terlalu dalam.
2. **Blok bertahan molor:** 35–38 vs 24–29 m. Penyerang dan AM tidak turun ke blok, garis bek terlalu dalam, jarak DM–AM 13 vs 7 m, kawalan di sepertiga akhir lebih longgar, dan sedikit terlalu sempit (29 vs 33.5 m).
3. **Sirkulasi bola:** CB/DM kurang dipakai, AM/ST kebanyakan. Ini kemungkinan besar akibat gap nomor 1.
4. **Intensitas gerak:** pemain go16 jalan 30% waktu (asli 48%) dan lari 4–7 m/s 27% (asli 13%).
5. **Koreksi berlebih yang perlu dibalik:** giring panjang (go13) dan pemisahan paksa FB (go14).

**Koreksi atas laporan sebelumnya:** cek cepat Metrica sebelum F0 bilang "bentuk menyerang go16 sudah mirip asli". Itu salah. Rata-rata semua zona dan penguasaan dari event menutupi gap di sepertiga sendiri dan tengah. Angka per zona di atas yang berlaku.

**Engine Rust r53** paling dekat di lebar, jalur pinggir, dan blok, tapi DM–AM-nya terlalu jauh (18 m). Player12 murni sempit dan panjang. Dua-duanya bukan target. Target = Metrica.

## Scorecard gabungan (`audit/scorecard.js`)

135 metrik bentuk dinilai sekaligus: pegang bola, bertahan, dan transisi di tiap zona, plus penerima bola, kecepatan, dan giring panjang. Penjaga = metrik cek lama.

| | Skor (makin kecil makin baik) | Lolos | Penjaga |
|---|---|---|---|
| Metrica g1 sendiri (cek kewajaran) | 0.24 | 135/135 | — |
| Rust r53 | 1.39 | 60/135 | — |
| **go16** | **1.49** | **54/135** | 4/11 |
| Player12 murni | 1.72 | 52/135 | — |

Metrik go16 yang paling jauh campur dari semua fase:
- tumpukan waktu transisi bertahan;
- garis belakang waktu transisi menyerang;
- lebar dan jalur pinggir waktu pegang bola;
- panjang blok bertahan (tenang dan transisi);
- jarak DM–AM dan AM–ST waktu bertahan;
- pemain jalan cuma 30% waktu (asli 48%).

Ini alasan semua fase dibangun bersama (PLAN §5.1).

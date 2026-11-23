# F1: Team Shape Controller + composer, mode bayangan (2026-11-23)

Bagian dari [PLAN-team-shape.md](PLAN-team-shape.md). **Gameplay tidak berubah:** go16 identik byte, golden hash tetap, `--reproduce` lolos. Slot cuma ditulis ke replay kalau `FMENGINE_SLOTS=1`.

## Yang dibangun

| Bagian | File | Isi |
|---|---|---|
| Model formasi | `engine/team_shape_table.go` (generated), `audit/fit_formation.js` | Per slot × 4 fase (pegang bola, bertahan, transisi pegang bola, transisi bertahan): polinomial posisi bola (8 suku) di frame tim. Di-fit dari Metrica dengan **penugasan peran dinamis per frame** (EM + Hungarian, Bialkowski dkk. 2014), plus faktor koreksi sebaran per fase (template rata-rata selalu lebih sempit dan pendek). |
| Team Shape Controller | `engine/team_shape.go` | Slot 22 pemain sekaligus (dua tim): fase dari penguasaan, bentuk transisi selama 4 detik lalu dicampur ke bentuk tenang dalam 1.5 detik, koreksi sebaran, frame tim (konvensi kiri/kanan engine dinormalisasi). Penugasan pemain→slot pakai Hungarian + histeresis (dipakai di F2). |
| Position Composer | `engine/compose.go` + `compose_test.go` | Tipe permintaan (job bola / berkuota / offset), kuota per tim, batas deviasi, jarak minimal, kelas kecepatan. Deterministik. 5 unit test. **Belum dipasang.** |
| Diagnostik | `Diagnostics.Slots`, `ReplayData.Slots`, CLI `FMENGINE_SLOTS=1` | Replay field `slots` (layout sama dengan pemain di `frames`) |
| Ukur | `audit/shape_suite.js --slots`, `audit/scorecard.js --slots` | Metrik bentuk dari slot; penguasaan tetap dari pemain |
| Viewer | `fm-game/viewer/viewer.html` (bukan repo ini) | Centang "slot": cincin per slot + garis dari pemain ke slotnya |

## Perubahan definisi ukur

Jarak antar baris (S4) dan kedalaman garis belakang (S6) sekarang **berbasis urutan kedalaman per frame**: 4 terdalam / 2 berikutnya / 3 berikutnya / paling depan. Sebelumnya pakai label peran tetap. Pemain asli tukar posisi, jadi label tetap mengaburkan garis di data asli tapi tidak di engine, dan perbandingannya tidak adil.

Dengan definisi baru, scorecard go16 aktual 1.41 (57/135). Di F0 tertulis 1.49 dengan definisi lama.

## Gerbang F1: scorecard slot (10 seed)

Di mode `--slots`, yang dinilai cuma **metrik struktur**: lebar, panjang, jarak antar baris, kedalaman garis belakang, di semua fase dan zona.

Metrik sebaran dan interaksi (tumpukan <4 m, jarak teman terdekat, jalur pinggir, kawalan) bergantung pada gerak pemain dan lawan. Jadi di sini cuma dicatat, dan dinilai pakai posisi aktual di F2.

| Iterasi | Skor (makin kecil makin baik) | Lolos |
|---|---|---|
| Rata-rata posisi per identitas pemain | 1.25 | 60/114 (semua metrik) |
| + penugasan peran dinamis (EM) | 0.90 | 79/114 (semua metrik) |
| Struktur saja, baris berbasis urutan | 0.62 | 62/72 |
| + model transisi, koreksi sebaran, transisi ditahan lalu dicampur | **0.52** | **63/72** |

Pegang bola tenang di tengah, Metrica / slot F1 / go16 aktual:

| | Metrica | slot F1 | go16 aktual |
|---|---|---|---|
| Lebar | 45.6 | 43.4 | 36.2 |
| Panjang | 29.7 | 29.0 | 25.8 |
| Belakang–pivot / pivot–gelandang / gelandang–depan | 11.4 / 9.4 / 5.6 | 10.1 / 10.9 / 4.5 | 8.5 / 7.9 / 8.4 |
| Jalur pinggir terisi | 71% | 82% | 28% |

Bertahan tenang di tengah:

| | Metrica | slot F1 | go16 aktual |
|---|---|---|---|
| Lebar | 33.5 | 32.9 | 29.0 |
| Panjang | 24.1 | 26.2 | 35.2 |
| Baris | 8.7 / 7.3 / 8.4 | 10.7 / 6.5 / 8.3 | 12.2 / 13.2 / 11.9 |

**Sisa yang belum lolos** (semua di bawah 1.4× toleransi):
- Transisi di sepertiga sendiri: setelah merebut bola, tim asli masih sempit dan dalam (lebar 31 vs slot 35, garis belakang 17 vs 22 m).
- Pegang bola tenang di sepertiga sendiri: jarak gelandang–depan 4 vs 7 m.
- Satu-dua metrik transisi di sepertiga akhir.

Waktu penguasaan berpindah di engine (pengendali terakhir) tidak persis sama dengan deteksi di replay (pemegang bola), jadi metrik transisi slot ada noise-nya.

**Biaya:** sekitar +0.25 dtk per laga (slot dihitung tiap sampel).

## Berikutnya: F2

Nyalakan semuanya bersamaan:
- posisi tenang dari TSC;
- penugasan dinamis;
- M1, P12, M5, M10 jadi permintaan ke composer;
- kelas kecepatan dari composer.

Lalu dinilai pakai scorecard aktual penuh (135 metrik + penjaga).

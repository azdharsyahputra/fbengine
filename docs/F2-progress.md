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

## Berikutnya

1. **Pembedahan pelari di momen chase dan bola liar** dalam mode native, lalu aturan composer siapa dan berapa orang yang boleh menuju bola.
2. **Penanda build-up untuk pivot lawan:** slot penyerang kita di zona press dekat DM lawan.
3. **Sapuan bersama parameter composer** dengan scorecard + penjaga.
4. **F4 (hapus jalur lama)** setelah scorecard dan penjaga lolos.

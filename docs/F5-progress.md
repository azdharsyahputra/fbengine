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

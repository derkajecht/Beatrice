# Crypto Library Benchmarking

A good issue was raised in regards to the rust overhead of the cryptography library. I wanted to test if there was any speed difference between [Cryptography](https://cryptography.io/en/latest/) which has a Rust backend, and [PyCryptodome](https://www.pycryptodome.org/) which has a C/C++ backend (correct me if I'm wrong).

---

Please find a link to the test program here;
[Benchmarks](https://github.com/derkajecht/Beatrice/blob/main/benchmarks/cryptolib_benchmark.py)

I tried to get this as close to the encryption happening inside Beatrice when sending and receiving messages. The program was run 10 times, here are the results;

| Library      | Best Time | Avg. |
| ------------ | --------- | ---- |
| Cryptography | 0.2521    | 0.4069     |
| PyCryptodome | 2.2818     | 3.29313     |

**Run 1** - Cryptography:   0.5283 seconds PyCryptodome:  3.9652 seconds

**Run 2** - Cryptography:   0.4442 seconds PyCryptodome:  2.2818 seconds

**Run 3** - Cryptography:   0.4118 seconds PyCryptodome:  3.4494 seconds

**Run 4** - Cryptography:   0.4888 seconds PyCryptodome:  2.5556 seconds

**Run 5** - Cryptography:   0.2521 seconds PyCryptodome:  3.4536 seconds

**Run 6** - Cryptography:   0.4949 seconds PyCryptodome:  3.2950 seconds

**Run 7** - Cryptography:   0.3139 seconds PyCryptodome:  3.2542 seconds

**Run 8** - Cryptography:   0.3455 seconds PyCryptodome:  4.5900 seconds

**Run 9** - Cryptography:   0.3475 seconds PyCryptodome:  3.3876 seconds

**Run 10** - Cryptography:   0.4417 seconds PyCryptodome:  2.6989 seconds

(module
  ;; ISAAC64 state: memory[256] at 0, results[256] at 2048.
  ;; The host transfer buffer starts at 4096 and is capped at 320 KiB.
  (memory (export "memory") 6 16)
  (global $a (mut i64) (i64.const 0))
  (global $b (mut i64) (i64.const 0))
  (global $c (mut i64) (i64.const 0))
  (global $count (mut i32) (i32.const 0))
  (global $word (mut i64) (i64.const 0))
  (global $word_byte (mut i32) (i32.const 8))
  (global $stream_offset (mut i64) (i64.const 0))

  (func $memory_address (param $index i32) (result i32)
    local.get $index
    i32.const 3
    i32.shl)

  (func $result_address (param $index i32) (result i32)
    local.get $index
    i32.const 3
    i32.shl
    i32.const 2048
    i32.add)

  (func $mix
    (param $a i64) (param $b i64) (param $c i64) (param $d i64)
    (param $e i64) (param $f i64) (param $g i64) (param $h i64)
    (result i64 i64 i64 i64 i64 i64 i64 i64)
    local.get $a local.get $e i64.sub local.set $a
    local.get $f local.get $h i64.const 9 i64.shr_u i64.xor local.set $f
    local.get $h local.get $a i64.add local.set $h
    local.get $b local.get $f i64.sub local.set $b
    local.get $g local.get $a i64.const 9 i64.shl i64.xor local.set $g
    local.get $a local.get $b i64.add local.set $a
    local.get $c local.get $g i64.sub local.set $c
    local.get $h local.get $b i64.const 23 i64.shr_u i64.xor local.set $h
    local.get $b local.get $c i64.add local.set $b
    local.get $d local.get $h i64.sub local.set $d
    local.get $a local.get $c i64.const 15 i64.shl i64.xor local.set $a
    local.get $c local.get $d i64.add local.set $c
    local.get $e local.get $a i64.sub local.set $e
    local.get $b local.get $d i64.const 14 i64.shr_u i64.xor local.set $b
    local.get $d local.get $e i64.add local.set $d
    local.get $f local.get $b i64.sub local.set $f
    local.get $c local.get $e i64.const 20 i64.shl i64.xor local.set $c
    local.get $e local.get $f i64.add local.set $e
    local.get $g local.get $c i64.sub local.set $g
    local.get $d local.get $f i64.const 17 i64.shr_u i64.xor local.set $d
    local.get $f local.get $g i64.add local.set $f
    local.get $h local.get $d i64.sub local.set $h
    local.get $e local.get $g i64.const 14 i64.shl i64.xor local.set $e
    local.get $g local.get $h i64.add local.set $g
    local.get $a local.get $b local.get $c local.get $d
    local.get $e local.get $f local.get $g local.get $h)

  (func $store_eight
    (param $index i32)
    (param $a i64) (param $b i64) (param $c i64) (param $d i64)
    (param $e i64) (param $f i64) (param $g i64) (param $h i64)
    local.get $index call $memory_address local.get $a i64.store
    local.get $index i32.const 1 i32.add call $memory_address local.get $b i64.store
    local.get $index i32.const 2 i32.add call $memory_address local.get $c i64.store
    local.get $index i32.const 3 i32.add call $memory_address local.get $d i64.store
    local.get $index i32.const 4 i32.add call $memory_address local.get $e i64.store
    local.get $index i32.const 5 i32.add call $memory_address local.get $f i64.store
    local.get $index i32.const 6 i32.add call $memory_address local.get $g i64.store
    local.get $index i32.const 7 i32.add call $memory_address local.get $h i64.store)

  (func $generate
    (local $index i32)
    (local $x i64)
    (local $y i64)
    global.get $c i64.const 1 i64.add global.set $c
    global.get $b global.get $c i64.add global.set $b
    block $done
      loop $items
        local.get $index i32.const 256 i32.ge_u br_if $done
        local.get $index call $memory_address i64.load local.set $x

        local.get $index i32.const 3 i32.and i32.eqz
        if
          global.get $a global.get $a i64.const 21 i64.shl i64.xor
          i64.const -1 i64.xor global.set $a
        end
        local.get $index i32.const 3 i32.and i32.const 1 i32.eq
        if
          global.get $a global.get $a i64.const 5 i64.shr_u i64.xor global.set $a
        end
        local.get $index i32.const 3 i32.and i32.const 2 i32.eq
        if
          global.get $a global.get $a i64.const 12 i64.shl i64.xor global.set $a
        end
        local.get $index i32.const 3 i32.and i32.const 3 i32.eq
        if
          global.get $a global.get $a i64.const 33 i64.shr_u i64.xor global.set $a
        end

        global.get $a
        local.get $index i32.const 128 i32.add i32.const 255 i32.and
        call $memory_address i64.load
        i64.add global.set $a

        local.get $x i64.const 3 i64.shr_u i32.wrap_i64 i32.const 255 i32.and
        call $memory_address i64.load
        global.get $a i64.add global.get $b i64.add local.set $y
        local.get $index call $memory_address local.get $y i64.store

        local.get $y i64.const 11 i64.shr_u i32.wrap_i64 i32.const 255 i32.and
        call $memory_address i64.load local.get $x i64.add global.set $b
        local.get $index call $result_address global.get $b i64.store

        local.get $index i32.const 1 i32.add local.set $index
        br $items
      end
    end)

  (func $initialise (param $seed i64)
    (local $index i32)
    (local $round i32)
    (local $a i64) (local $b i64) (local $c i64) (local $d i64)
    (local $e i64) (local $f i64) (local $g i64) (local $h i64)
    i32.const 2048 local.get $seed i64.store
    i64.const 0x9e3779b97f4a7c13 local.tee $a local.tee $b local.tee $c local.tee $d
    local.tee $e local.tee $f local.tee $g local.set $h

    block $mixed
      loop $warmup
        local.get $round i32.const 4 i32.ge_u br_if $mixed
        local.get $a local.get $b local.get $c local.get $d
        local.get $e local.get $f local.get $g local.get $h call $mix
        local.set $h local.set $g local.set $f local.set $e
        local.set $d local.set $c local.set $b local.set $a
        local.get $round i32.const 1 i32.add local.set $round
        br $warmup
      end
    end

    block $first_done
      loop $first_pass
        local.get $index i32.const 256 i32.ge_u br_if $first_done
        local.get $a local.get $index call $result_address i64.load i64.add local.set $a
        local.get $b local.get $index i32.const 1 i32.add call $result_address i64.load i64.add local.set $b
        local.get $c local.get $index i32.const 2 i32.add call $result_address i64.load i64.add local.set $c
        local.get $d local.get $index i32.const 3 i32.add call $result_address i64.load i64.add local.set $d
        local.get $e local.get $index i32.const 4 i32.add call $result_address i64.load i64.add local.set $e
        local.get $f local.get $index i32.const 5 i32.add call $result_address i64.load i64.add local.set $f
        local.get $g local.get $index i32.const 6 i32.add call $result_address i64.load i64.add local.set $g
        local.get $h local.get $index i32.const 7 i32.add call $result_address i64.load i64.add local.set $h
        local.get $a local.get $b local.get $c local.get $d
        local.get $e local.get $f local.get $g local.get $h call $mix
        local.set $h local.set $g local.set $f local.set $e
        local.set $d local.set $c local.set $b local.set $a
        local.get $index local.get $a local.get $b local.get $c local.get $d
        local.get $e local.get $f local.get $g local.get $h call $store_eight
        local.get $index i32.const 8 i32.add local.set $index
        br $first_pass
      end
    end

    i32.const 0 local.set $index
    block $second_done
      loop $second_pass
        local.get $index i32.const 256 i32.ge_u br_if $second_done
        local.get $a local.get $index call $memory_address i64.load i64.add local.set $a
        local.get $b local.get $index i32.const 1 i32.add call $memory_address i64.load i64.add local.set $b
        local.get $c local.get $index i32.const 2 i32.add call $memory_address i64.load i64.add local.set $c
        local.get $d local.get $index i32.const 3 i32.add call $memory_address i64.load i64.add local.set $d
        local.get $e local.get $index i32.const 4 i32.add call $memory_address i64.load i64.add local.set $e
        local.get $f local.get $index i32.const 5 i32.add call $memory_address i64.load i64.add local.set $f
        local.get $g local.get $index i32.const 6 i32.add call $memory_address i64.load i64.add local.set $g
        local.get $h local.get $index i32.const 7 i32.add call $memory_address i64.load i64.add local.set $h
        local.get $a local.get $b local.get $c local.get $d
        local.get $e local.get $f local.get $g local.get $h call $mix
        local.set $h local.set $g local.set $f local.set $e
        local.set $d local.set $c local.set $b local.set $a
        local.get $index local.get $a local.get $b local.get $c local.get $d
        local.get $e local.get $f local.get $g local.get $h call $store_eight
        local.get $index i32.const 8 i32.add local.set $index
        br $second_pass
      end
    end
    call $generate
    i32.const 256 global.set $count
    i32.const 8 global.set $word_byte
    i64.const 0 global.set $stream_offset)

  (func $next_word (result i64)
    global.get $count i32.eqz
    if
      call $generate
      i32.const 256 global.set $count
    end
    global.get $count i32.const 1 i32.sub global.set $count
    global.get $count call $result_address i64.load)

  (func (export "rd_abi_version") (result i32) i32.const 1)

  (func (export "rd_alloc") (param $size i32) (result i32)
    local.get $size i32.const 327680 i32.le_u
    if (result i32) i32.const 4096 else i32.const 0 end)

  (func (export "rd_free") (param $pointer i32) (param $size i32))

  (func (export "rd_init") (param $pointer i32) (param $length i32) (result i32)
    (local $index i32) (local $end i32) (local $character i32) (local $seed i64)
    local.get $pointer local.set $index
    local.get $pointer local.get $length i32.add local.set $end
    block $colon_found
      loop $find_colon
        local.get $index local.get $end i32.ge_u
        if i32.const -1 return end
        local.get $index i32.load8_u i32.const 58 i32.eq br_if $colon_found
        local.get $index i32.const 1 i32.add local.set $index
        br $find_colon
      end
    end
    local.get $index i32.const 1 i32.add local.set $index
    block $digit_found
      loop $find_digit
        local.get $index local.get $end i32.ge_u
        if i32.const -2 return end
        local.get $index i32.load8_u local.tee $character
        i32.const 48 i32.ge_u
        local.get $character i32.const 57 i32.le_u i32.and
        br_if $digit_found
        local.get $index i32.const 1 i32.add local.set $index
        br $find_digit
      end
    end
    block $digits_done
      loop $parse_digits
        local.get $index local.get $end i32.ge_u br_if $digits_done
        local.get $index i32.load8_u local.tee $character
        i32.const 48 i32.lt_u br_if $digits_done
        local.get $character i32.const 57 i32.gt_u br_if $digits_done
        local.get $seed i64.const 10 i64.mul
        local.get $character i32.const 48 i32.sub i64.extend_i32_u i64.add local.set $seed
        local.get $index i32.const 1 i32.add local.set $index
        br $parse_digits
      end
    end
    local.get $seed call $initialise
    i32.const 0)

  (func (export "rd_transform")
    (param $pointer i32) (param $input_length i32) (param $capacity i32)
    (param $offset_low i32) (param $offset_high i32) (param $final i32)
    (result i32)
    (local $index i32) (local $limit i32) (local $remaining i32)
    (local $offset i64) (local $shift i64) (local $key i32)
    local.get $input_length local.get $capacity i32.gt_u
    if i32.const -1 return end
    local.get $offset_high i64.extend_i32_u i64.const 32 i64.shl
    local.get $offset_low i64.extend_i32_u i64.or local.set $offset
    local.get $offset i64.const 131072 i64.ge_u
    if local.get $input_length return end
    global.get $stream_offset local.get $offset i64.gt_u
    if i32.const -2 return end
    block $skip_done
      loop $skip
        global.get $stream_offset local.get $offset i64.ge_u br_if $skip_done
        global.get $word_byte i32.const 8 i32.ge_u
        if
          call $next_word global.set $word
          i32.const 0 global.set $word_byte
        end
        global.get $word_byte i32.const 1 i32.add global.set $word_byte
        global.get $stream_offset i64.const 1 i64.add global.set $stream_offset
        br $skip
      end
    end
    i64.const 131072 local.get $offset i64.sub i32.wrap_i64 local.set $remaining
    local.get $input_length local.set $limit
    local.get $remaining local.get $limit i32.lt_u
    if local.get $remaining local.set $limit end
    block $done
      loop $bytes
        local.get $index local.get $limit i32.ge_u br_if $done
        global.get $word_byte i32.const 8 i32.ge_u
        if
          call $next_word global.set $word
          i32.const 0 global.set $word_byte
        end
        i32.const 7 global.get $word_byte i32.sub i64.extend_i32_u
        i64.const 3 i64.shl local.set $shift
        global.get $word local.get $shift i64.shr_u i32.wrap_i64 i32.const 255 i32.and local.set $key
        local.get $pointer local.get $index i32.add
        local.get $pointer local.get $index i32.add i32.load8_u local.get $key i32.xor i32.store8
        global.get $word_byte i32.const 1 i32.add global.set $word_byte
        global.get $stream_offset i64.const 1 i64.add global.set $stream_offset
        local.get $index i32.const 1 i32.add local.set $index
        br $bytes
      end
    end
    local.get $input_length)
)

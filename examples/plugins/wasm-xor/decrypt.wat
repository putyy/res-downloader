(module
  ;; The host streams at most 256 KiB and reserves another 64 KiB for output.
  (memory (export "memory") 6 16)
  (global $key (mut i32) (i32.const 0))

  (func (export "rd_abi_version") (result i32)
    i32.const 1)

  ;; This demo reuses a fixed 320 KiB region beginning at byte 1024.
  (func (export "rd_alloc") (param $size i32) (result i32)
    local.get $size
    i32.const 327680
    i32.le_u
    if (result i32)
      i32.const 1024
    else
      i32.const 0
    end)

  (func (export "rd_free") (param $pointer i32) (param $size i32))

  ;; The example expects the host options JSON to contain a numeric `key`.
  (func (export "rd_init") (param $pointer i32) (param $length i32) (result i32)
    (local $index i32)
    (local $end i32)
    (local $value i32)
    (local $digit i32)
    local.get $pointer
    local.set $index
    local.get $pointer
    local.get $length
    i32.add
    local.set $end

    ;; Find the first colon in {"key":90}.
    block $colon_found
      loop $find_colon
        local.get $index
        local.get $end
        i32.ge_u
        if
          i32.const -1
          return
        end
        local.get $index
        i32.load8_u
        i32.const 58
        i32.eq
        br_if $colon_found
        local.get $index
        i32.const 1
        i32.add
        local.set $index
        br $find_colon
      end
    end
    local.get $index
    i32.const 1
    i32.add
    local.set $index

    ;; Skip JSON whitespace, then parse an unsigned decimal byte.
    block $digit_found
      loop $skip
        local.get $index
        local.get $end
        i32.ge_u
        if
          i32.const -2
          return
        end
        local.get $index
        i32.load8_u
        local.tee $digit
        i32.const 48
        i32.ge_u
        local.get $digit
        i32.const 57
        i32.le_u
        i32.and
        br_if $digit_found
        local.get $index
        i32.const 1
        i32.add
        local.set $index
        br $skip
      end
    end

    block $digits_done
      loop $parse_digits
        local.get $index
        local.get $end
        i32.ge_u
        br_if $digits_done
        local.get $index
        i32.load8_u
        local.tee $digit
        i32.const 48
        i32.lt_u
        br_if $digits_done
        local.get $digit
        i32.const 57
        i32.gt_u
        br_if $digits_done
        local.get $value
        i32.const 10
        i32.mul
        local.get $digit
        i32.const 48
        i32.sub
        i32.add
        local.set $value
        local.get $index
        i32.const 1
        i32.add
        local.set $index
        br $parse_digits
      end
    end
    local.get $value
    i32.const 255
    i32.gt_u
    if
      i32.const -3
      return
    end
    local.get $value
    global.set $key
    i32.const 0)

  (func (export "rd_transform")
    (param $pointer i32)
    (param $input_length i32)
    (param $capacity i32)
    (param $offset_low i32)
    (param $offset_high i32)
    (param $final i32)
    (result i32)
    (local $index i32)
    local.get $input_length
    local.get $capacity
    i32.gt_u
    if
      i32.const -1
      return
    end
    block $done
      loop $xor
        local.get $index
        local.get $input_length
        i32.ge_u
        br_if $done
        local.get $pointer
        local.get $index
        i32.add
        local.get $pointer
        local.get $index
        i32.add
        i32.load8_u
        global.get $key
        i32.xor
        i32.store8
        local.get $index
        i32.const 1
        i32.add
        local.set $index
        br $xor
      end
    end
    local.get $input_length)
)

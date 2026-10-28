(module
  (memory (export "memory") 1)
  (func $spin
    loop $forever
      br $forever
    end)
  (start $spin)
)

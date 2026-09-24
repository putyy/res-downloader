import {nextTick} from 'vue'
import {EventsEmit} from '../../wailsjs/runtime'

let revealScheduled = false

export const revealStartupWindow = async () => {
    if (revealScheduled) return
    revealScheduled = true
    await nextTick()

    let revealed = false
    let frame = 0
    const reveal = () => {
        if (revealed) return
        revealed = true
        clearTimeout(fallback)
        cancelAnimationFrame(frame)
        // Flush the initial styles before enabling subsequent theme transitions.
        document.body.getBoundingClientRect()
        document.documentElement.classList.remove('app-starting')
        EventsEmit('window:ready')
    }

    // Hidden webviews may suspend animation frames; never depend on them alone.
    const fallback = setTimeout(reveal, 120)
    frame = requestAnimationFrame(() => {
        frame = requestAnimationFrame(reveal)
    })
}

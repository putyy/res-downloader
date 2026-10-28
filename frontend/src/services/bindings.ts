// Wails v3 generates strongly typed bindings for every registered service.
// Re-exporting them here keeps application code independent of the generated
// package layout, which follows the Go module path.
export {Bind} from "@bindings/res-downloader/internal/app"

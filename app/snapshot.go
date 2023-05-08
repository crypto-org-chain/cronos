package app

import (
	"os"
	"path/filepath"
	"strconv"

	abci "github.com/tendermint/tendermint/abci/types"
)

// OfferSnapshot implements the ABCI interface. It delegates to app.snapshotManager if set.
func (app *App) OfferSnapshot(req abci.RequestOfferSnapshot) abci.ResponseOfferSnapshot {
	if app.saveSnapshotDir != "" {
		bz, err := req.Marshal()
		if err != nil {
			panic(err)
		}
		if err := os.WriteFile(filepath.Join(app.saveSnapshotDir, "snapshot"), bz, 0644); err != nil {
			panic(err)
		}
	}
	return app.BaseApp.OfferSnapshot(req)
}

// ApplySnapshotChunk implements the ABCI interface. It delegates to app.snapshotManager if set.
func (app *App) ApplySnapshotChunk(req abci.RequestApplySnapshotChunk) abci.ResponseApplySnapshotChunk {
	if app.saveSnapshotDir != "" {
		if err := os.WriteFile(filepath.Join(app.saveSnapshotDir, strconv.FormatUint(uint64(req.Index), 10)), req.Chunk, 0644); err != nil {
			panic(err)
		}
	}
	return app.BaseApp.ApplySnapshotChunk(req)
}

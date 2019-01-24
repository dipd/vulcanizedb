// Copyright 2018 Vulcanize
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package chop_lump

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/vulcanize/vulcanizedb/pkg/datastore/postgres"
	"github.com/vulcanize/vulcanizedb/pkg/transformers/shared"
	"github.com/vulcanize/vulcanizedb/pkg/transformers/shared/constants"
)

type CatFileChopLumpRepository struct {
	db *postgres.DB
}

func (repository CatFileChopLumpRepository) Create(headerID int64, models []interface{}) error {
	tx, dBaseErr := repository.db.Begin()
	if dBaseErr != nil {
		return dBaseErr
	}

	for _, model := range models {
		chopLump, ok := model.(CatFileChopLumpModel)
		if !ok {
			rollbackErr := tx.Rollback()
			if rollbackErr != nil {
				log.Error("failed to rollback ", rollbackErr)
			}
			return fmt.Errorf("model of type %T, not %T", model, CatFileChopLumpModel{})
		}

		_, execErr := tx.Exec(
			`INSERT into maker.cat_file_chop_lump (header_id, ilk, what, data, tx_idx, log_idx, raw_log)
			VALUES($1, $2, $3, $4::NUMERIC, $5, $6, $7)`,
			headerID, chopLump.Ilk, chopLump.What, chopLump.Data, chopLump.TransactionIndex, chopLump.LogIndex, chopLump.Raw,
		)
		if execErr != nil {
			rollbackErr := tx.Rollback()
			if rollbackErr != nil {
				log.Error("failed to rollback ", rollbackErr)
			}
			return execErr
		}
	}

	checkHeaderErr := shared.MarkHeaderCheckedInTransaction(headerID, tx, constants.CatFileChopLumpChecked)
	if checkHeaderErr != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			log.Error("failed to rollback ", rollbackErr)
		}
		return checkHeaderErr
	}
	return tx.Commit()
}

func (repository CatFileChopLumpRepository) MarkHeaderChecked(headerID int64) error {
	return shared.MarkHeaderChecked(headerID, repository.db, constants.CatFileChopLumpChecked)
}

func (repository *CatFileChopLumpRepository) SetDB(db *postgres.DB) {
	repository.db = db
}
// VulcanizeDB
// Copyright © 2019 Vulcanize

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.

// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package repository

import "github.com/jmoiron/sqlx"

func MarkContractWatcherHeaderCheckedInTransaction(headerID int64, tx *sqlx.Tx, checkedHeadersColumn string) error {
	_, err := tx.Exec(`INSERT INTO public.checked_headers (header_id, `+checkedHeadersColumn+`)
		VALUES ($1, $2)
		ON CONFLICT (header_id) DO
		UPDATE SET `+checkedHeadersColumn+` = checked_headers.`+checkedHeadersColumn+` + 1`, headerID, 1)
	return err
}
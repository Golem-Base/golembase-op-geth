package entity

//go:generate protoc --proto_path=proto --go_out=. --go_opt=paths=source_relative proto/storageutil/entity/active_payload.proto
//go:generate go run ../../../rlp/rlpgen -type EntityMetaData -out gen_entity_meta_data_rlp.go

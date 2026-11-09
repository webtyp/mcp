
| Test (file) | setup → call → expected |
|---|---|
| TestToolNameOf_IsModuleEntityVerb (tests/tool_name_test.go) | mcp.ToolNameOf(router.Operation{Module:"booking", Entity:"reservation", Action: model.Read})=="booking.reservation.read"; Create→…create, Update→…update, Delete→…delete |
| TestHarvest_ToolPerAllowedAction | a module with one table allowing Create\|Read → HarvestOps(nil, [create, read tools], mod) returns two tools; exposing the update tool → error "is not registered by any module" |
| TestHarvest_WriteToolPublishesAfterCommit | module with a HaveNoBusinessRule Create table over memStore, pub := &recorder{}; HarvestOps(pub, [create], mod); run the tool with {"records":[{"id":"a","name":"x"}]} → len(pub.got)==1, pub.got[0].Topic==crud.Topic("fake", "fake_record", model.Create) |
| TestHarvest_NilPublisherIsFine | same module, pub == nil → the tool runs and stores, no panic |
| TestCaller_SendsTheQualifiedToolName (tests/caller_name_test.go) | NewCaller(client).Call(router.Operation{Module:"fake", Entity:"fake_record", Action: model.Read}, nil, &out, done) with the local client of local_client_test.go → the server handled the tool fake.fake_record.read |

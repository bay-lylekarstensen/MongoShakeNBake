package reconcile

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	utils "github.com/alibaba/MongoShake/v2/common"
	LOG "github.com/alibaba/MongoShake/v2/third_party/log4go"
)

type NamespacePair struct {
	Source utils.NS
	Target utils.NS
}

type Options struct {
	TargetURL        string
	SourceURLs       []string
	Namespaces       []NamespacePair
	TargetSSLRoot    string
	SourceSSLRoot    string
	ShadowDB         string
	ShadowCollection string
	Interval         time.Duration
	DeleteBatchSize  int
	GraceRuns        int
}

type Service struct {
	opts Options
}

func NewService(opts Options) *Service {
	return &Service{opts: opts}
}

func (s *Service) Start() {
	go func() {
		if err := s.RunOnce(); err != nil {
			_ = LOG.Warn("reconcile run failed: %v", err)
		}

		ticker := time.NewTicker(s.opts.Interval)
		defer ticker.Stop()

		for range ticker.C {
			if err := s.RunOnce(); err != nil {
				_ = LOG.Warn("reconcile run failed: %v", err)
			}
		}
	}()
}

func (s *Service) RunOnce() error {
	if len(s.opts.Namespaces) == 0 || len(s.opts.SourceURLs) == 0 {
		return nil
	}

	start := time.Now()
	runID := start.UnixNano()
	LOG.Info("reconcile run start run_id[%d] namespaces[%d]", runID, len(s.opts.Namespaces))

	targetConn, err := utils.NewMongoCommunityConn(s.opts.TargetURL, utils.VarMongoConnectModePrimary, true,
		utils.ReadWriteConcernDefault, utils.ReadWriteConcernDefault, s.opts.TargetSSLRoot)
	if err != nil {
		return fmt.Errorf("connect target for reconcile failed: %w", err)
	}
	defer targetConn.Close()

	sourceConns := make(map[string]*utils.MongoCommunityConn, len(s.opts.SourceURLs))
	for _, sourceURL := range s.opts.SourceURLs {
		if _, ok := sourceConns[sourceURL]; ok {
			continue
		}
		conn, connErr := utils.NewMongoCommunityConn(sourceURL, utils.VarMongoConnectModeSecondaryPreferred, true,
			utils.ReadWriteConcernDefault, utils.ReadWriteConcernDefault, s.opts.SourceSSLRoot)
		if connErr != nil {
			for _, c := range sourceConns {
				c.Close()
			}
			return fmt.Errorf("connect source for reconcile failed: %w", connErr)
		}
		sourceConns[sourceURL] = conn
	}
	defer func() {
		for _, c := range sourceConns {
			c.Close()
		}
	}()

	for _, pair := range s.opts.Namespaces {
		if err := s.reconcileNamespace(targetConn.Client, sourceConns, pair, runID); err != nil {
			return err
		}
	}

	LOG.Info("reconcile run done run_id[%d] cost[%s]", runID, time.Since(start))
	return nil
}

type stateDoc struct {
	ID          interface{} `bson:"_id"`
	LastSeenRun int64       `bson:"last_seen_run"`
	MissingRuns int         `bson:"missing_runs"`
}

func (s *Service) reconcileNamespace(targetClient *mongo.Client, sourceConns map[string]*utils.MongoCommunityConn,
	pair NamespacePair, runID int64) error {
	stateColl := targetClient.Database(s.opts.ShadowDB).Collection(buildStateCollectionName(s.opts.ShadowCollection, pair.Target))
	targetColl := targetClient.Database(pair.Target.Database).Collection(pair.Target.Collection)

	for _, sourceConn := range sourceConns {
		sourceColl := sourceConn.Client.Database(pair.Source.Database).Collection(pair.Source.Collection)
		if err := s.markSeenFromSource(stateColl, sourceColl, runID); err != nil {
			return fmt.Errorf("reconcile mark seen failed ns[%s] source[%s]: %w", pair.Target.Str(), pair.Source.Str(), err)
		}
	}

	if err := s.pruneMissing(stateColl, targetColl, runID); err != nil {
		return fmt.Errorf("reconcile prune failed ns[%s]: %w", pair.Target.Str(), err)
	}

	return nil
}

func (s *Service) markSeenFromSource(stateColl *mongo.Collection, sourceColl *mongo.Collection, runID int64) error {
	ctx := context.Background()
	cursor, err := sourceColl.Find(ctx, bson.D{}, options.Find().SetProjection(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	ops := make([]mongo.WriteModel, 0, s.opts.DeleteBatchSize)
	flush := func() error {
		if len(ops) == 0 {
			return nil
		}
		_, flushErr := stateColl.BulkWrite(ctx, ops, options.BulkWrite().SetOrdered(false))
		ops = ops[:0]
		return flushErr
	}

	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			return err
		}
		id, ok := doc["_id"]
		if !ok {
			continue
		}

		ops = append(ops, mongo.NewUpdateOneModel().
			SetFilter(bson.D{{Key: "_id", Value: id}}).
			SetUpdate(bson.D{{Key: "$set", Value: bson.D{
				{Key: "last_seen_run", Value: runID},
				{Key: "missing_runs", Value: 0},
				{Key: "updated_at", Value: time.Now().UTC()},
			}}}).
			SetUpsert(true))

		if len(ops) >= s.opts.DeleteBatchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}

	if err := cursor.Err(); err != nil {
		return err
	}

	return flush()
}

func (s *Service) pruneMissing(stateColl, targetColl *mongo.Collection, runID int64) error {
	ctx := context.Background()
	cursor, err := targetColl.Find(ctx, bson.D{}, options.Find().SetProjection(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	type idItem struct {
		id  interface{}
		key string
	}

	batch := make([]idItem, 0, s.opts.DeleteBatchSize)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}

		ids := make([]interface{}, 0, len(batch))
		for _, item := range batch {
			ids = append(ids, item.id)
		}

		stateCursor, err := stateColl.Find(ctx, bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}}},
			options.Find().SetProjection(bson.D{
				{Key: "_id", Value: 1},
				{Key: "last_seen_run", Value: 1},
				{Key: "missing_runs", Value: 1},
			}))
		if err != nil {
			return err
		}

		stateByID := make(map[string]stateDoc, len(batch))
		for stateCursor.Next(ctx) {
			var doc stateDoc
			if err := stateCursor.Decode(&doc); err != nil {
				_ = stateCursor.Close(ctx)
				return err
			}
			stateByID[marshalIDKey(doc.ID)] = doc
		}
		if err := stateCursor.Err(); err != nil {
			_ = stateCursor.Close(ctx)
			return err
		}
		_ = stateCursor.Close(ctx)

		stateOps := make([]mongo.WriteModel, 0, len(batch))
		deleteOps := make([]mongo.WriteModel, 0)
		stateDeleteOps := make([]mongo.WriteModel, 0)

		for _, item := range batch {
			state, ok := stateByID[item.key]
			if ok && state.LastSeenRun == runID {
				continue
			}

			missing := 1
			if ok {
				missing = state.MissingRuns + 1
			}

			if missing >= s.opts.GraceRuns {
				deleteOps = append(deleteOps, mongo.NewDeleteOneModel().SetFilter(bson.D{{Key: "_id", Value: item.id}}))
				stateDeleteOps = append(stateDeleteOps, mongo.NewDeleteOneModel().SetFilter(bson.D{{Key: "_id", Value: item.id}}))
				continue
			}

			stateOps = append(stateOps, mongo.NewUpdateOneModel().
				SetFilter(bson.D{{Key: "_id", Value: item.id}}).
				SetUpdate(bson.D{{Key: "$set", Value: bson.D{
					{Key: "missing_runs", Value: missing},
					{Key: "updated_at", Value: time.Now().UTC()},
				}}}).
				SetUpsert(true))
		}

		if len(stateOps) > 0 {
			if _, err := stateColl.BulkWrite(ctx, stateOps, options.BulkWrite().SetOrdered(false)); err != nil {
				return err
			}
		}
		if len(deleteOps) > 0 {
			if _, err := targetColl.BulkWrite(ctx, deleteOps, options.BulkWrite().SetOrdered(false)); err != nil {
				return err
			}
			if _, err := stateColl.BulkWrite(ctx, stateDeleteOps, options.BulkWrite().SetOrdered(false)); err != nil {
				return err
			}
		}

		batch = batch[:0]
		return nil
	}

	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			return err
		}
		id, ok := doc["_id"]
		if !ok {
			continue
		}

		batch = append(batch, idItem{id: id, key: marshalIDKey(id)})
		if len(batch) >= s.opts.DeleteBatchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}

	if err := cursor.Err(); err != nil {
		return err
	}

	return flush()
}

func buildStateCollectionName(prefix string, ns utils.NS) string {
	name := strings.ReplaceAll(ns.Str(), ".", "__")
	name = strings.ReplaceAll(name, "$", "_")
	return prefix + "__" + name
}

func marshalIDKey(id interface{}) string {
	data, err := bson.Marshal(bson.D{{Key: "_id", Value: id}})
	if err != nil {
		return fmt.Sprintf("%v", id)
	}
	return string(data)
}

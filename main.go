package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/XSAM/otelsql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/overstarry/ent-trace/ent"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

func tracerProvider(url string) (*tracesdk.TracerProvider, error) {
	// Create the Jaeger exporter
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(url)))
	if err != nil {
		return nil, err
	}
	tp := tracesdk.NewTracerProvider(
		tracesdk.WithSampler(tracesdk.AlwaysSample()),
		tracesdk.WithBatcher(exp),
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("opentelemetry-ent-trace"), // 服务名
			semconv.ServiceVersionKey.String("0.0.1"),
			attribute.String("environment", "test"),
		)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return tp, nil
}

func main() {
	_, err := tracerProvider("http://localhost:14268/api/traces")
	if err != nil {
		log.Fatal(err)
	}
	driverName, err := otelsql.Register("mysql", semconv.DBSystemMySQL.Value.AsString())
	if err != nil {
		panic(err)
	}

	// Connect to database
	db, err := sql.Open(driverName, "root:a12345@tcp(127.0.0.1:3306)/trace?parseTime=True")
	if err != nil {
		panic(err)
	}
	defer db.Close()
	drv := entsql.OpenDB("mysql", db)
	client := ent.NewClient(ent.Driver(drv))
	// Run the auto migration tool.
	if err := client.Schema.Create(context.Background()); err != nil {
		log.Fatalf("failed creating schema resources: %v", err)
	}
	createUser(client)
	http.ListenAndServe(":8080", nil)
}

func createUser(client *ent.Client) {
	tracer := otel.GetTracerProvider()
	ctx, span := tracer.Tracer("github.com/overstarry/ent-trace/example").Start(context.Background(), "example")
	defer span.End()

	u, err := client.User.Create().
		SetName("Overstarry").Save(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Created: %v", u)
}

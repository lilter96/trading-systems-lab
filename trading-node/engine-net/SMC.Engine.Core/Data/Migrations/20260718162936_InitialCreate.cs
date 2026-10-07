using System;
using Microsoft.EntityFrameworkCore.Migrations;
using Npgsql.EntityFrameworkCore.PostgreSQL.Metadata;

#nullable disable

namespace SMC.Engine.Core.Data.Migrations
{
    /// <inheritdoc />
    public partial class InitialCreate : Migration
    {
        /// <inheritdoc />
        protected override void Up(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.CreateTable(
                name: "strategy_config",
                columns: table => new
                {
                    id = table.Column<Guid>(type: "uuid", nullable: false),
                    name = table.Column<string>(type: "character varying(100)", maxLength: 100, nullable: false),
                    value = table.Column<string>(type: "jsonb", nullable: false),
                    description = table.Column<string>(type: "text", nullable: true),
                    updated_at = table.Column<DateTimeOffset>(type: "timestamp with time zone", nullable: false)
                },
                constraints: table =>
                {
                    table.PrimaryKey("PK_strategy_config", x => x.id);
                });

            migrationBuilder.CreateTable(
                name: "trading_sessions",
                columns: table => new
                {
                    id = table.Column<Guid>(type: "uuid", nullable: false),
                    strategy_name = table.Column<string>(type: "character varying(100)", maxLength: 100, nullable: false),
                    started_at = table.Column<DateTimeOffset>(type: "timestamp with time zone", nullable: false),
                    ended_at = table.Column<DateTimeOffset>(type: "timestamp with time zone", nullable: true),
                    initial_balance = table.Column<decimal>(type: "numeric(18,8)", nullable: false),
                    current_equity = table.Column<decimal>(type: "numeric(18,8)", nullable: false),
                    status = table.Column<string>(type: "character varying(20)", maxLength: 20, nullable: false),
                    created_at = table.Column<DateTimeOffset>(type: "timestamp with time zone", nullable: false)
                },
                constraints: table =>
                {
                    table.PrimaryKey("PK_trading_sessions", x => x.id);
                });

            migrationBuilder.CreateTable(
                name: "audit_log",
                columns: table => new
                {
                    id = table.Column<long>(type: "bigint", nullable: false)
                        .Annotation("Npgsql:ValueGenerationStrategy", NpgsqlValueGenerationStrategy.IdentityByDefaultColumn),
                    session_id = table.Column<Guid>(type: "uuid", nullable: false),
                    event_type = table.Column<string>(type: "character varying(50)", maxLength: 50, nullable: false),
                    correlation_id = table.Column<Guid>(type: "uuid", nullable: true),
                    payload = table.Column<string>(type: "jsonb", nullable: false),
                    created_at = table.Column<DateTimeOffset>(type: "timestamp with time zone", nullable: false)
                },
                constraints: table =>
                {
                    table.PrimaryKey("PK_audit_log", x => x.id);
                    table.ForeignKey(
                        name: "FK_audit_log_trading_sessions_session_id",
                        column: x => x.session_id,
                        principalTable: "trading_sessions",
                        principalColumn: "id",
                        onDelete: ReferentialAction.Restrict);
                });

            migrationBuilder.CreateTable(
                name: "orders",
                columns: table => new
                {
                    id = table.Column<Guid>(type: "uuid", nullable: false),
                    session_id = table.Column<Guid>(type: "uuid", nullable: false),
                    exchange_order_id = table.Column<string>(type: "character varying(100)", maxLength: 100, nullable: true),
                    asset = table.Column<string>(type: "character varying(20)", maxLength: 20, nullable: false),
                    direction = table.Column<string>(type: "character varying(10)", maxLength: 10, nullable: false),
                    size = table.Column<decimal>(type: "numeric(18,8)", nullable: false),
                    status = table.Column<string>(type: "character varying(20)", maxLength: 20, nullable: false),
                    execution_price = table.Column<decimal>(type: "numeric(18,8)", nullable: true),
                    order_type = table.Column<string>(type: "character varying(20)", maxLength: 20, nullable: false),
                    limit_price = table.Column<decimal>(type: "numeric(18,8)", nullable: true),
                    stop_price = table.Column<decimal>(type: "numeric(18,8)", nullable: true),
                    created_at = table.Column<DateTimeOffset>(type: "timestamp with time zone", nullable: false),
                    updated_at = table.Column<DateTimeOffset>(type: "timestamp with time zone", nullable: false),
                    filled_at = table.Column<DateTimeOffset>(type: "timestamp with time zone", nullable: true),
                    rejected_reason = table.Column<string>(type: "text", nullable: true)
                },
                constraints: table =>
                {
                    table.PrimaryKey("PK_orders", x => x.id);
                    table.ForeignKey(
                        name: "FK_orders_trading_sessions_session_id",
                        column: x => x.session_id,
                        principalTable: "trading_sessions",
                        principalColumn: "id",
                        onDelete: ReferentialAction.Restrict);
                });

            migrationBuilder.CreateTable(
                name: "positions",
                columns: table => new
                {
                    id = table.Column<Guid>(type: "uuid", nullable: false),
                    session_id = table.Column<Guid>(type: "uuid", nullable: false),
                    asset = table.Column<string>(type: "character varying(20)", maxLength: 20, nullable: false),
                    direction = table.Column<string>(type: "character varying(10)", maxLength: 10, nullable: false),
                    entry_price = table.Column<decimal>(type: "numeric(18,8)", nullable: false),
                    current_size = table.Column<decimal>(type: "numeric(18,8)", nullable: false),
                    stop_loss = table.Column<decimal>(type: "numeric(18,8)", nullable: true),
                    take_profit = table.Column<decimal>(type: "numeric(18,8)", nullable: true),
                    status = table.Column<string>(type: "character varying(20)", maxLength: 20, nullable: false),
                    opened_at = table.Column<DateTimeOffset>(type: "timestamp with time zone", nullable: false),
                    closed_at = table.Column<DateTimeOffset>(type: "timestamp with time zone", nullable: true),
                    realized_pnl = table.Column<decimal>(type: "numeric(18,8)", nullable: true)
                },
                constraints: table =>
                {
                    table.PrimaryKey("PK_positions", x => x.id);
                    table.ForeignKey(
                        name: "FK_positions_trading_sessions_session_id",
                        column: x => x.session_id,
                        principalTable: "trading_sessions",
                        principalColumn: "id",
                        onDelete: ReferentialAction.Restrict);
                });

            migrationBuilder.CreateTable(
                name: "risk_limits",
                columns: table => new
                {
                    id = table.Column<Guid>(type: "uuid", nullable: false),
                    session_id = table.Column<Guid>(type: "uuid", nullable: true),
                    daily_loss_limit = table.Column<decimal>(type: "numeric(18,8)", nullable: false),
                    max_open_positions = table.Column<int>(type: "integer", nullable: false),
                    global_halt = table.Column<bool>(type: "boolean", nullable: false),
                    max_position_size = table.Column<decimal>(type: "numeric(18,8)", nullable: true),
                    max_leverage = table.Column<decimal>(type: "numeric(10,2)", nullable: true),
                    updated_at = table.Column<DateTimeOffset>(type: "timestamp with time zone", nullable: false)
                },
                constraints: table =>
                {
                    table.PrimaryKey("PK_risk_limits", x => x.id);
                    table.ForeignKey(
                        name: "FK_risk_limits_trading_sessions_session_id",
                        column: x => x.session_id,
                        principalTable: "trading_sessions",
                        principalColumn: "id",
                        onDelete: ReferentialAction.SetNull);
                });

            migrationBuilder.CreateIndex(
                name: "IX_audit_log_session_id",
                table: "audit_log",
                column: "session_id");

            migrationBuilder.CreateIndex(
                name: "IX_orders_session_id",
                table: "orders",
                column: "session_id");

            migrationBuilder.CreateIndex(
                name: "IX_positions_session_id",
                table: "positions",
                column: "session_id");

            migrationBuilder.CreateIndex(
                name: "IX_risk_limits_session_id",
                table: "risk_limits",
                column: "session_id");

            migrationBuilder.CreateIndex(
                name: "IX_strategy_config_name",
                table: "strategy_config",
                column: "name",
                unique: true);
        }

        /// <inheritdoc />
        protected override void Down(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.DropTable(
                name: "audit_log");

            migrationBuilder.DropTable(
                name: "orders");

            migrationBuilder.DropTable(
                name: "positions");

            migrationBuilder.DropTable(
                name: "risk_limits");

            migrationBuilder.DropTable(
                name: "strategy_config");

            migrationBuilder.DropTable(
                name: "trading_sessions");
        }
    }
}

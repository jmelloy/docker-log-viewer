# Notion Export for SQL Queries

This feature allows you to export SQL query analysis reports directly to Notion.

## Setup

### 1. Create a Notion Integration

1. Go to [https://www.notion.so/my-integrations](https://www.notion.so/my-integrations)
2. Click "+ New integration"
3. Give it a name (e.g., "Docker Log Viewer SQL Export")
4. Select the workspace where you want to export data
5. Click "Submit"
6. Copy the "Internal Integration Token" - this is your `NOTION_API_KEY`

### 2. Create a Notion Database

1. In Notion, create a new page or use an existing one
2. Add a database (table or any view)
3. Make sure it has at least a "Name" property (title field)
4. Click the "..." menu in the top right of your database
5. Click "Add connections" and select your integration
6. Copy the database ID from the URL:
   - URL format: `https://www.notion.so/{workspace}/{database_id}?v={view_id}`
   - The `database_id` is the 32-character string after the workspace name
   - Example: If URL is `https://www.notion.so/myworkspace/abc123def456?v=xyz`, the database ID is `abc123def456`

### 3. Configure Environment Variables

Set the following environment variables before running the Docker Log Viewer:

```bash
export NOTION_API_KEY="secret_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
export NOTION_DATABASE_ID="abc123def456abc123def456abc123de"
```

Or add them to your `.env` file or docker-compose.yml:

```yaml
# docker-compose.yml
services:
  viewer:
    environment:
      - NOTION_API_KEY=${NOTION_API_KEY}
      - NOTION_DATABASE_ID=${NOTION_DATABASE_ID}
```

## Usage

1. Navigate to a SQL query detail page in the Docker Log Viewer
2. Click the "📘 Export to Notion" button
3. The page will be created in your configured Notion database
4. A success message will show the URL of the created page
5. Click the link to open the page in Notion

## What Gets Exported

The Notion page includes:

- **Query Information**
  - Operation type (SELECT, INSERT, UPDATE, DELETE)
  - Table name
  - Total executions
  - Average, min, and max duration
  - Request ID (if available)
  - Last execution date

- **SQL Query**
  - Formatted SQL query in a code block

- **Normalized Query**
  - Normalized version (with placeholders) in a code block

- **EXPLAIN Plan** (if available)
  - PostgreSQL EXPLAIN plan output in a code block

## Markdown Export

If you don't want to use Notion, you can export the same information as a Markdown file:

1. Click the "📄 Export Markdown" button
2. A `.md` file will be downloaded to your default downloads folder
3. You can open it in any Markdown editor or viewer

The Markdown file includes all the same information as the Notion export, formatted with:
- Headers and sections
- Code blocks for SQL queries
- Tables for related executions
- Bullet points for metadata

## Troubleshooting

### "Notion API key not configured" Error

Make sure you've set the `NOTION_API_KEY` environment variable before starting the application.

### "Notion database ID not configured" Error

Make sure you've set the `NOTION_DATABASE_ID` environment variable before starting the application.

### "Failed to create Notion page" Error

Check that:
1. Your Notion integration has access to the database
2. The database ID is correct (32 characters, no dashes)
3. The database has a "Name" property (title field)
4. Your API key is valid and not expired

### Page Created but Can't See It

1. Check that you're looking at the correct database
2. Try refreshing the database view
3. Check if there are any filters applied to your database view
4. Make sure your integration has "Read" and "Insert" permissions

## API Endpoint

The Notion export is handled by a POST request to:

```
POST /api/sql/{queryHash}/export-notion
```

Response:
```json
{
  "url": "https://www.notion.so/page-id",
  "message": "Successfully exported to Notion"
}
```

Error response:
```json
{
  "error": "Error message"
}
```

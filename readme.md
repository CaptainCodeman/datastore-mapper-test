# datastore-mapper-test

Simple project used for testing performance and correctness of
[appengine datastore-mapper](https://github.com/captaincodeman/datastore-mapper)

## Generate test data

Test data is generated using seed values so the 'random' entries are repeatable.
This allows the same data to be generated locally to sum up values and check them
against the totals ultimately from BigQuery.

The test data is simple Order entities with one or more Order Items.

POST to /_ah/start?start=0&finish=10000000

Use start + finish to set the range of data to generate. new_only=true will make
the system query for data and only write new entries for any gaps found.

Each entity records the seed ID it was created with. The datastore key is based on
a hashid of this integer which produces some distribution of keys so inserts are
not all sequential.

## Run mapper jobs

There are two mapper jobs defined. When running them it's important to understand
how sharding and concurrent request limits affect processing.

Shards are how many slices the datastore dataset is sliced into. Only one request
per shard will ever be running at the same time so this controls how much work can
be done in parallel.

The queue.yaml max_concurrent_requests setting controls how many shard requests
will be executed concurrently.

The app.yaml max_concurrent_requests setting controls how many shard requests can
be processed by a single instance.

All together, these allow you to control the 'scale out' of the mapper job and how
much work is executed concurrently and how many instances will be fired up to do it.

Of course more + bigger instances will be faster ... but more expensive.

### Export to JSON

This job exports the datastore entities to a json file. The bucket to write to is
required which should be owned by the project:

POST /_ah/mapper/start?name=main.ExportJson&bucket=mapper-perf.appspot.com&shards=16

GCS writing is done in buffered chunks so there is some overhead for each shard that
is processed concurrently by any one instance. I found that an F2 instance could run
4 requests concurrently and used around 200Mb of RAM.

For 50 million entities and no effective limit to the number of concurrent shards:

 16 shards took around 80 minutes to complete.
 32 shards took around 40 minutes to complete.
 64 shards took around 20 minutes to complete.

On average each 10-minute shard request processed around 400,000 - 450,000 entities
which works out to roughly 700 - 750 entities per second per shard.

The operation consumed:

    17.94 Instance Hours
    50.03 Million Datastore Read Ops

Costs were around $30 with the majority being datastore read operations. This could
be reduced by doing a 'KeysOnly' iteration of the entity keys and loading them from
memcache using quedus\nds although its effectiveness would depend on the proportion
of data is typically cached. For frequent batch-updates of new data it could be an
effective strategy.

All produced a ~30Gb json file and importing this into BigQuery took approximately
2 minutes. The schema.json is the BigQuery scheme definition used to define the table
format.

F4 instance (512Mb RAM)

8-900,000 per shard request

10 concurrent ~360Mb
16 concurrent ~

Suggested max concurrent requests per instance type for JSON exporting

Class  Memory  Max
F1      128Mb    2
F2      256Mb    6
F4      512Mb   16
F4_1G  1024Mb   32

TODO: Explain balance between memory use, performance and cost

### Export to BigQuery

As an alternative, I also created a job to export directly to BigQuery using streaming
inserts. These are much slower and the chance of duplicate data being inserted is
higher.

However, even though some task operations were restarted at some point, the InsertID
feature of the streaming inserts did it's job and the final table had exactly 50,000,000
entries, exactly the same as per the JSON export / ingestion approach.

The memory overhead was considerably lower and an instance could easily handle 8 - 16
concurrent shards at once.

Even though I batched the BigQuery writes, the streaming insert approach was considerably
slower though with the overall job taking days (although that was with fewer shards).

This approach really makes more sense for having live updates of data or smaller repeat
batches (e.g. hourly or daily). This would require a CRON task to create the query for
the previous period range and an appropriate index on the datastore (e.g. a date field
with no time information so an equality filter could be used in the query). Also, a
composite index would be needed on the date + `__scatter__` special property used for
splitting the query range into shards.

The table for the BigQuery export is created automatically in the example.

## Mapper Options

TODO: Effect of other mapper options such as request timeout
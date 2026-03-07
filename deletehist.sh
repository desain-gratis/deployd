for run in $(gh run list --limit 100 --json databaseId -q '.[].databaseId'); do
  gh run delete $run
done

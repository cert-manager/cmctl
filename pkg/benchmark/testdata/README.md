# README

To run a benchmark experiment:

```sh
export experiment_id=YYYY-MM-DD-INDEX
mkdir -p experiment.${experiment_id}
cd experiment.${experiment_id}

touch values.yaml
# edit values.yaml with the cert-manager parameters for the experiment

../setup.sh
cmctl x benchmark > experiment.${experiment_id}.json
jq -s -r 'include "module-cm"; recordsToCSV'  < experiment.${experiment_id}.json  > experiment.${experiment_id}.csv
```

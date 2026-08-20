package messaging

import (
	"fmt"
	"slices"
)

// Catalog 是 composition 聚合后的不可变消息声明快照。
type Catalog struct {
	contracts []Contract
	producers []ProducerBinding
	consumers []ConsumerBinding
}

// BuildCatalog 聚合模块贡献并执行全局 identity、引用与 schema 冲突校验。
func BuildCatalog(contributions ...Contribution) (Catalog, error) {
	contracts := make(map[ContractRef]Contract)
	producers := make(map[ProducerID]ProducerBinding)
	consumers := make(map[ConsumerID]ConsumerBinding)
	uses := make(map[ContractRef]int)
	for _, contribution := range contributions {
		for _, contract := range contribution.contracts {
			if err := contract.validate(); err != nil {
				return Catalog{}, err
			}
			if current, exists := contracts[contract.ref]; exists && !current.sameDefinition(contract) {
				return Catalog{}, fmt.Errorf("%w: %s", ErrContractConflict, contract.ref)
			}
			contracts[contract.ref] = contract
		}
		for _, producer := range contribution.producers {
			if err := producer.validate(); err != nil {
				return Catalog{}, err
			}
			if _, exists := producers[producer.id]; exists {
				return Catalog{}, fmt.Errorf("%w: producer %s", ErrDuplicateBinding, producer.id)
			}
			producers[producer.id] = producer
			uses[producer.contract]++
		}
		for _, consumer := range contribution.consumers {
			if err := consumer.validate(); err != nil {
				return Catalog{}, err
			}
			if _, exists := consumers[consumer.id]; exists {
				return Catalog{}, fmt.Errorf("%w: consumer %s", ErrDuplicateBinding, consumer.id)
			}
			consumers[consumer.id] = consumer
			uses[consumer.contract]++
		}
	}
	for ref := range uses {
		if _, exists := contracts[ref]; !exists {
			return Catalog{}, fmt.Errorf("%w: %s", ErrUnknownContract, ref)
		}
	}
	for ref := range contracts {
		if uses[ref] == 0 {
			return Catalog{}, fmt.Errorf("%w: %s", ErrUnusedContract, ref)
		}
	}
	catalog := Catalog{
		contracts: make([]Contract, 0, len(contracts)), producers: make([]ProducerBinding, 0, len(producers)),
		consumers: make([]ConsumerBinding, 0, len(consumers)),
	}
	for _, value := range contracts {
		catalog.contracts = append(catalog.contracts, value)
	}
	for _, value := range producers {
		catalog.producers = append(catalog.producers, value)
	}
	for _, value := range consumers {
		catalog.consumers = append(catalog.consumers, value)
	}
	slices.SortFunc(catalog.contracts, func(a, b Contract) int { return compareRef(a.ref, b.ref) })
	slices.SortFunc(catalog.producers, func(a, b ProducerBinding) int { return compareString(string(a.id), string(b.id)) })
	slices.SortFunc(catalog.consumers, func(a, b ConsumerBinding) int { return compareString(string(a.id), string(b.id)) })
	return catalog, nil
}

// Contracts 返回排序后的 Contract 副本。
func (c Catalog) Contracts() []Contract { return append([]Contract(nil), c.contracts...) }

// Producers 返回排序后的 Producer Binding 副本。
func (c Catalog) Producers() []ProducerBinding { return append([]ProducerBinding(nil), c.producers...) }

// Consumers 返回排序后的 Consumer Binding 副本。
func (c Catalog) Consumers() []ConsumerBinding { return append([]ConsumerBinding(nil), c.consumers...) }

func compareRef(a, b ContractRef) int {
	if compared := compareString(string(a.id), string(b.id)); compared != 0 {
		return compared
	}
	if a.version < b.version {
		return -1
	}
	if a.version > b.version {
		return 1
	}
	return 0
}

func compareString(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

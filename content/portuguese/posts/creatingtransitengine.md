---
date: '2026-01-10T20:01:00-03:00'
draft: false
title: 'Construindo o PathCraft: Um Motor de Roteamento Open-Source em Go'
tags:
  - algoritmos
  - projetos
  - go
---

## Introdução

Quando comecei a construir o **PathCraft**, tinha um objetivo simples: criar um motor de roteamento que eu pudesse realmente entender, estender e implantar sem vendor lock-in. O que começou como um projeto experimental evoluiu para um motor de roteamento multimodal e modular que lida com tudo, desde navegação de pedestres até roteamento de transporte público.

Neste post, vou compartilhar a jornada de construção do PathCraft. As decisões arquiteturais, os algoritmos que o impulsionam e as lições aprendidas ao longo do caminho.

---

## O Que é o PathCraft?

PathCraft é um motor experimental de roteamento para caminhada e transporte público escrito em Go. Ele foi projetado para ser:

- Uma **biblioteca Go reutilizável (SDK)** que desenvolvedores podem integrar em suas aplicações
- Uma **aplicação CLI** para consultas rápidas de rotas
- Um **servidor HTTP** para implantações em produção
- Um **motor embarcável** para integração em sistemas maiores

A filosofia central? **Correção primeiro, performance depois.** Muitos motores de roteamento sacrificam legibilidade e manutenibilidade por ganhos marginais de performance. O PathCraft adota uma abordagem diferente.

---

## A Stack Tecnológica

### Por Que Go?

Go foi uma escolha natural para este projeto:

1. **Performance**: A natureza compilada do Go nos dá performance quase nativa sem sacrificar produtividade
2. **Simplicidade**: O design minimalista da linguagem se alinha com a filosofia do PathCraft de clareza sobre complexidade
3. **Concorrência**: Goroutines e channels nativos tornam roteamento paralelo uma possibilidade futura
4. **Binário Único**: Deploy fácil—apenas envie um binário, sem dependências de runtime

### Componentes Principais

- **Implementação Customizada de A\***: Feita do zero para máximo controle e valor educacional
- **Algoritmo RAPTOR**: Para roteamento eficiente de transporte público
- **Parser OSM**: Ingere dados do OpenStreetMap para construir grafos caminháveis
- **Parser GTFS**: Processa dados de especificação GTFS para horários de transporte
- **Cálculos de Haversine**: Computações de distância geográfica para heurísticas

---

## Arquitetura: O Monolito Modular

Uma das decisões mais importantes foi a arquitetura. Inicialmente considerei uma arquitetura Hexagonal (Ports and Adapters), mas acabei escolhendo uma abordagem de **Monolito Modular**.

### Por Que Não Hexagonal?

Para um projeto focado em correção algorítmica e iteração rápida, arquitetura Hexagonal introduz overhead desnecessário. As camadas de indireção e fronteiras de abstração, embora valiosas para grandes sistemas empresariais, desacelerariam o desenvolvimento sem fornecer benefícios proporcionais.

### A Vantagem do Monolito Modular

A base de código é organizada em módulos bem definidos com fronteiras explícitas:

```
/internal         → Lógica central privada
  /graph          → Estruturas de dados de grafos
  /geo            → Cálculos geográficos
  /routing        → Algoritmos A*, RAPTOR
  /osm            → Parsing do OpenStreetMap
  /gtfs           → Parsing de dados de transporte

/pkg/pathcraft/engine  → API pública (única coisa que usuários importam)
/cmd/pathcraft         → Entrypoint CLI
/web                   → Ferramentas de visualização
```

Esta estrutura mantém baixo acoplamento e alta coesão enquanto mantém o sistema fácil de entender. O fluxo de dependências é rigoroso: **a lógica central nunca depende de infraestrutura** (HTTP, CLI, I/O de arquivos).

---

## Deep Dive: O Algoritmo A\*

No coração do roteador de caminhada do PathCraft está o algoritmo A\*. Aqui está uma visão simplificada de como funciona:

### A Ideia Central

A\* combina o melhor do algoritmo de Dijkstra (caminho mais curto garantido) com busca guiada por heurística (exploração mais rápida). Para cada nó, calculamos:

```
f(n) = g(n) + h(n)
```

Onde:
- **g(n)**: Custo real do início até o nó n
- **h(n)**: Custo estimado de n até o objetivo (nossa heurística)

### A Heurística: Distância de Haversine

Para roteamento geográfico, usamos a fórmula de Haversine para calcular a distância do círculo máximo entre dois pontos na Terra. Isso nos dá uma heurística admissível—nunca superestima a distância real, garantindo que encontremos o caminho ideal.

### Implementação da Fila de Prioridade

O algoritmo mantém um conjunto aberto como fila de prioridade, sempre explorando o nó mais promissor primeiro. O pacote `container/heap` do Go fornece a base, com uma struct customizada `pqItem` rastreando IDs de nós e prioridades.

---

## Deep Dive: RAPTOR para Roteamento de Transporte

Para transporte público, implementamos o algoritmo **RAPTOR** (Round-bAsed Public Transit Optimized Router). Diferente de abordagens tradicionais baseadas em grafos, RAPTOR trabalha diretamente com dados de horários.

### Como o RAPTOR Funciona

1. **Rounds**: Cada round representa uma perna adicional de transporte (transferência)
2. **Varredura de Rotas**: Para cada parada marcada, varre todas as rotas que servem aquela parada
3. **Processamento de Transferências**: Após varredura de rotas, processa transferências a pé
4. **Otimalidade de Pareto**: Rastreia tanto tempo de chegada quanto número de transferências

### Por Que RAPTOR?

- **Velocidade**: Tipicamente mais rápido que abordagens baseadas em grafos para transporte
- **Simplicidade**: O algoritmo é elegante e fácil de entender
- **Flexibilidade**: Fácil adicionar restrições (max transferências, distância caminhável)

---

## O Facade do Engine

A struct `Engine` em `/pkg/pathcraft/engine` serve como API pública—o único pacote que usuários externos devem importar.

```go
type Engine struct {
    graph     *graph.Graph
    gtfsIndex *gtfs.StopTimeIndex
}

// Métodos principais
func (e *Engine) Route(req RouteRequest) (RouteResult, error)
func (e *Engine) LoadOSM(path string) error
func (e *Engine) LoadGTFS(stopTimesPath, tripsPath string) error
```

O engine **orquestra**, não computa. Esta separação mantém preocupações de HTTP/CLI longe dos algoritmos.

---

## Lidando com Tempo GTFS: O Problema >24:00:00

Um desafio interessante foi lidar com formatos de tempo GTFS. No GTFS, horários podem exceder 24:00:00 para representar serviços que continuam após meia-noite. Uma viagem partindo às `25:30:00` significa 1:30 AM do dia seguinte.

Construímos um pacote customizado `time` em `/internal/time` que lida com este caso extremo de forma transparente, mantendo o resto da base de código limpa.

---

## Princípios de Desenvolvimento

Durante o desenvolvimento, aderi a vários princípios:

### 1. Núcleo Puro e Determinístico

Os algoritmos de roteamento são funções puras. Dado o mesmo grafo e consulta, sempre produzem o mesmo resultado. Sem estado oculto, sem aleatoriedade, sem efeitos colaterais.

### 2. Testabilidade Primeiro

Cada algoritmo de roteamento tem testes abrangentes. Casos extremos são obrigatórios. Isso não é apenas boa prática—é essencial quando você está implementando algoritmos onde bugs podem ser sutis e difíceis de detectar.

### 3. Sem Abstração Prematura

Resisti ao impulso de abstrair muito cedo. Interfaces são introduzidas apenas quando há uma necessidade concreta, não baseadas em requisitos futuros hipotéticos.

### 4. Comentários Explicam Por Quê, Não O Quê

O código deve ser auto-documentado para *o que* ele faz. Comentários são reservados para explicar *por que* certas decisões foram tomadas.

---

## Status Atual & Roadmap

### Concluído

- Parsing de OSM e construção de grafos
- Roteamento A\* para caminhada
- Interface CLI
- Export GeoJSON
- Modo servidor HTTP com endpoints `/route` e `/health`
- Ingestão GTFS
- Implementação do algoritmo RAPTOR
- Visualização básica de mapas

### Em Progresso

- Integração Caminhada + Transporte
- Roteamento dependente de tempo
- Benchmarking de performance

### Planos Futuros

- Contração de grafos para consultas mais rápidas
- Estratégias de cache
- Bindings WASM/JavaScript
- API gRPC
- Sistema de plugins

---

## Lições Aprendidas

### 1. Comece Simples, Itere Rápido

A primeira versão do PathCraft era embaraçosamente simples. Isso foi intencional. Faça algo funcionar, depois melhore.

### 2. Invista em Ferramentas Cedo

Um bom Makefile, formatação consistente e testes automatizados pagam dividendos. Cada hora gasta em ferramentas economiza dias de debugging depois.

### 3. Documentação como Design

Escrever documentação força você a pensar sobre seu design. Se você não consegue explicar de forma simples, provavelmente não entende bem o suficiente.

### 4. Algoritmos São a Parte Fácil

As implementações de A\* e RAPTOR foram diretas. As partes difíceis? Parsing de dados, casos extremos e fazer tudo funcionar junto perfeitamente.

---

## Experimente Você Mesmo

PathCraft é open source e está disponível no GitHub. Para começar:

```bash
# Clone o repositório
git clone https://github.com/danielscoffee/pathcraft

# Build da CLI
make build

# Veja comandos disponíveis
./bin/pathcraft --help
```

Você precisará de um arquivo OSM para sua área de interesse. O diretório `examples/` contém dados de exemplo para começar.

---

## Conclusão

Construir o PathCraft tem sido uma experiência de aprendizado incrível. Desde implementar algoritmos clássicos até projetar interfaces limpas, cada desafio me ensinou algo novo.

O objetivo permanece o mesmo do primeiro dia:

> "O motor de roteamento open-source que você implanta quando não quer vendor lock-in."

Se você tem interesse em algoritmos de roteamento, desenvolvimento em Go, ou apenas quer entender como aplicativos de navegação funcionam por baixo dos panos, espero que o PathCraft possa servir tanto como ferramenta útil quanto como recurso educacional.

Contribuições são bem-vindas. Bom roteamento!

## Leituras Complementares

- [Algoritmo de Busca A\* - Wikipedia](https://en.wikipedia.org/wiki/A*_search_algorithm)
- [Paper RAPTOR - Microsoft Research](https://www.microsoft.com/en-us/research/wp-content/uploads/2012/01/raptor_alenex.pdf)
- [OpenStreetMap](https://www.openstreetmap.org/)
- [Especificação GTFS](https://gtfs.org/)

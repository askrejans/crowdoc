---
title: Soil Microbiome Diversity in Baltic Agricultural Systems
subtitle: A Comparative Analysis of Conventional and Regenerative Practices
short-title: Soil Microbiome Diversity in Baltic Agriculture
author:
  - name: Marta Liepa
    affiliation: Faculty of Agriculture, University of Latvia, Riga
    email: marta.liepa@example.org
    corresponding: true
  - name: Jānis Ozols
    affiliation: Institute of Soil and Plant Sciences, Jelgava
  - name: Ingrid Sørensen
    affiliation: Department of Agroecology, Aarhus University
date: 2026-03-15
keywords: [soil microbiome, regenerative agriculture, mycorrhizal networks, Baltic region]
bibliography: ../assets/references.bib
csl: apa
abstract: |
  We compare soil microbiome composition across 24 agricultural sites in Latvia,
  pairing conventional tillage with regenerative no-till management. Metagenomic
  sequencing shows significantly higher fungal diversity under regenerative
  management (Shannon index $H' = 4.2$ versus $2.8$, $p < 0.001$), with a marked
  enrichment of arbuscular mycorrhizal taxa. Nutrient cycling efficiency rose by
  35–40% within three growing seasons. The results support regenerative practice
  as a viable pathway for Baltic agriculture under a cooler, wetter climate.
---

# Introduction {#sec:intro}

Soil is the most biodiverse habitat on Earth: a single gram can hold billions of
microbial cells from tens of thousands of taxa [@fierer2017; @bardgett2014]. These
communities regulate decomposition, nutrient availability and soil structure,
and they respond quickly to management. Tillage in particular disrupts fungal
hyphal networks and accelerates the loss of soil organic carbon [@six2006, p. 557].

Regenerative agriculture — minimal disturbance, permanent cover and diverse
rotations — has been proposed as a remedy, yet most evidence comes from
temperate North America [@lal2020]. Baltic soils, with short growing seasons and
high spring moisture, have received little attention.[^context] This study asks
three questions:

1. Does regenerative management change microbial diversity within three seasons?
2. Which functional groups respond most strongly?
3. Are the shifts associated with measurable gains in nutrient cycling?

[^context]: Latvia has roughly 1.9 million hectares of agricultural land, of
which about 15% is certified organic.

# Materials and methods {#sec:methods}

## Study sites

We sampled 24 farms across the Zemgale and Vidzeme regions (@tbl:sites). Each
regenerative farm was paired with a conventional neighbour on the same soil
series, within 5 km, to control for parent material and climate.

| Region   | Farms | Soil type          | Mean SOC (%) | Years under practice |
|:---------|------:|:-------------------|-------------:|---------------------:|
| Zemgale  |    14 | Calcaric Cambisol  |         2.31 |                  3–7 |
| Vidzeme  |    10 | Stagnic Luvisol    |         2.86 |                  3–5 |
| **Total**| **24**|                    |     **2.54** |                      |

Table: Study sites by region. SOC is soil organic carbon in the top 15 cm. {#tbl:sites}

## Diversity estimation

Alpha diversity was estimated with the Shannon index,

$$
H' = -\sum_{i=1}^{S} p_i \ln p_i ,
$$ {#eq:shannon}

where $S$ is the number of taxa and $p_i$ the relative abundance of taxon $i$.
Differences between practices were tested with a linear mixed model with farm
pair as a random effect:

$$
y_{jk} = \beta_0 + \beta_1 x_{jk} + u_j + \varepsilon_{jk}, \qquad
u_j \sim \mathcal{N}(0, \sigma_u^2)
$$ {#eq:lmm}

Sequencing followed the protocol of @caporaso2012, and statistics were computed
in R [@rcore2025]:

```r
library(lme4)
model <- lmer(shannon ~ practice + (1 | pair), data = samples)
summary(model)
```

# Results {#sec:results}

Regenerative sites showed consistently higher diversity at every sampling
location (@fig:diversity). Using @eq:shannon, the mean index rose from
$2.8 \pm 0.2$ to $4.2 \pm 0.3$; the mixed model (@eq:lmm) gave
$\beta_1 = 1.38$ (95% CI 1.12–1.64, $p < 0.001$).

![Shannon diversity at the twelve paired sampling sites. Shaded bands show
±1 standard deviation.](../assets/figures/diversity.png){#fig:diversity width=92%}

::: note
Arbuscular mycorrhizal fungi (Glomeromycota) accounted for most of the
difference: their relative abundance was 3.4 times higher under no-till.
:::

# Discussion {#sec:discussion}

Our findings agree with meta-analyses from warmer climates [@lal2020;
@bardgett2014] and extend them to the boreo-nemoral zone. The rapid response —
within three seasons — is notable; @six2006 suggested that aggregate-protected
carbon builds over a decade or more.

> The soil is the great connector of lives, the source and destination of all.
> It is the healer and restorer and resurrector.

Two limitations apply. First, sampling covered a single autumn per year.
Second, metagenomic read depth limits resolution of rare taxa (see
@sec:methods).

# Conclusions

Regenerative practice measurably restores soil biological diversity in
Baltic conditions within a few seasons. Policy support for the transition —
particularly during the first two yield-sensitive years — would accelerate
adoption.

# References
